package collector

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/tukesh1/git-brief/internal/config"
)

// Matches GitHub-style merge commits, e.g.
//
//	Merge pull request #6 from Tukesh1/slack-integration
var prMergeSubject = regexp.MustCompile(`(?i)^Merge pull request #(\d+) from\s+(\S+)`)

// CommitData holds information about a single git commit.
type CommitData struct {
	Repo    string
	Message string
	Branch  string
	Date    string
}

// Warning is a non-fatal informational message produced during collection.
type Warning string

// resolveTilde expands a leading ~ to the current user's home directory.
func resolveTilde(filePath string) string {
	if filePath == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(filePath, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, filePath[2:])
	}
	abs, _ := filepath.Abs(filePath)
	return abs
}

// isGitRepo reports whether path is inside a git work tree.
func isGitRepo(ctx context.Context, path string) bool {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// currentBranch returns the current branch name for the repo at path,
// or "(detached)" if HEAD is detached.
func currentBranch(ctx context.Context, path string) string {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return "(unknown)"
	}
	b := strings.TrimSpace(string(out))
	if b == "HEAD" {
		return "(detached)"
	}
	return b
}

// findGitRepos walks baseDirs (one level deep) and returns unique git repos.
func findGitRepos(ctx context.Context, baseDirs []string) ([]string, []Warning) {
	seen := make(map[string]bool)
	var repos []string
	var warnings []Warning

	addRepo := func(path string) {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			real = path
		}
		if !seen[real] {
			seen[real] = true
			repos = append(repos, path)
		}
	}

	for _, baseDir := range baseDirs {
		baseDir = resolveTilde(baseDir)
		if _, err := os.Stat(baseDir); os.IsNotExist(err) {
			warnings = append(warnings, Warning(fmt.Sprintf("directory does not exist: %s", baseDir)))
			continue
		}

		if isGitRepo(ctx, baseDir) {
			addRepo(baseDir)
			continue
		}

		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			subDir := filepath.Join(baseDir, entry.Name())
			if isGitRepo(ctx, subDir) {
				addRepo(subDir)
			}
		}
	}
	return repos, warnings
}

// getGitIdent resolves the git author identity (name, email) to filter commits.
// Priority: config.Cfg.Author/Email → git config user.name/email.
func getGitIdent(ctx context.Context) (string, string) {
	name := config.Cfg.Author
	email := config.Cfg.Email

	if name == "" {
		if out, err := exec.CommandContext(ctx, "git", "config", "user.name").Output(); err == nil {
			name = strings.TrimSpace(string(out))
		}
	}
	if email == "" {
		if out, err := exec.CommandContext(ctx, "git", "config", "user.email").Output(); err == nil {
			email = strings.TrimSpace(string(out))
		}
	}
	return name, email
}

// getSinceArg returns the git --since= value.
func getSinceArg(override string, days int) string {
	if override != "" {
		return override
	}
	return config.SinceTime(days).Format("2006-01-02T15:04:05")
}

// CollectResult holds the output of a git data collection run.
type CollectResult struct {
	Commits     []CommitData
	MergedPRs   []PRData // from local "Merge pull request #N" commits (no GitHub token needed)
	Uncommitted []string // list of repos with uncommitted changes
	Stashed     []string // list of recent stashes
	Warnings    []Warning
	Repos       int
}

// CollectGitData reads commits from the configured/given workspaces.
func CollectGitData(ctx context.Context, since string, days int, workspaces []string) CollectResult {
	var result CollectResult

	authorNameCfg, authorEmailCfg := getGitIdent(ctx)
	if authorNameCfg == "" && authorEmailCfg == "" {
		result.Warnings = append(result.Warnings, Warning("could not determine git author name/email — commits may not be filtered correctly"))
	}

	var baseDirs []string
	switch {
	case len(workspaces) > 0:
		baseDirs = workspaces
	case len(config.Cfg.Workspaces) > 0:
		baseDirs = config.Cfg.Workspaces
	default:
		cwd, _ := os.Getwd()
		baseDirs = []string{cwd}
	}

	repos, warnings := findGitRepos(ctx, baseDirs)
	result.Warnings = append(result.Warnings, warnings...)
	result.Repos = len(repos)

	if len(repos) == 0 {
		result.Warnings = append(result.Warnings, Warning("no git repositories found in the provided workspaces"))
		return result
	}

	sinceArg := getSinceArg(since, days)

	for _, repoPath := range repos {
		branch := currentBranch(ctx, repoPath)
		repoName := filepath.Base(repoPath)

		// Fetch all commits using a custom delimiter format:
		// \x1e (record separator) starts a commit.
		// \x1f (unit separator) separates fields: Subject, CommitDate, AuthorName, AuthorEmail, Body
		args := []string{"log", "--all", "--since=" + sinceArg, "--no-merges", "--format=\x1e%s\x1f%cI\x1f%an\x1f%ae\x1f%b"}
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = repoPath
		out, err := cmd.Output()
		if err == nil {
			blocks := strings.Split(string(out), "\x1e")
			for _, block := range blocks {
				if strings.TrimSpace(block) == "" {
					continue
				}
				fields := strings.Split(strings.TrimSpace(block), "\x1f")
				if len(fields) >= 4 {
					subject := fields[0]
					commitDate := fields[1]
					authorName := fields[2]
					authorEmail := fields[3]
					body := ""
					if len(fields) >= 5 {
						body = fields[4]
					}

					// Check if user is author OR co-author
					isAuthor := authorName == authorNameCfg || authorEmail == authorEmailCfg
					isCoAuthor := authorEmailCfg != "" && strings.Contains(body, "Co-authored-by: ") && strings.Contains(body, authorEmailCfg)
					isCoAuthorName := authorNameCfg != "" && strings.Contains(body, "Co-authored-by: ") && strings.Contains(body, authorNameCfg)

					if isAuthor || isCoAuthor || isCoAuthorName {
						result.Commits = append(result.Commits, CommitData{
							Repo:    repoName,
							Message: subject,
							Branch:  branch,
							Date:    commitDate,
						})
					}
				}
			}
		}

		// Local PR merges are excluded by --no-merges above. Capture them so a
		// merged PR still appears in the standup without a GitHub API token.
		result.MergedPRs = append(result.MergedPRs, collectLocalMergedPRs(ctx, repoPath, repoName, sinceArg, authorNameCfg, authorEmailCfg)...)

		// Check for uncommitted changes (skip tooling/build noise paths).
		statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
		statusCmd.Dir = repoPath
		statusOut, err := statusCmd.Output()
		if err == nil {
			lines := strings.Split(string(bytes.TrimSpace(statusOut)), "\n")
			var raw []string
			for _, line := range lines {
				if line == "" {
					continue
				}
				path := line
				if len(line) > 3 {
					path = strings.TrimSpace(line[3:]) // skip status prefix
				}
				// Renames look like "old -> new"
				if i := strings.Index(path, " -> "); i >= 0 {
					path = path[i+4:]
				}
				raw = append(raw, path)
			}
			files := MeaningfulFiles(raw)
			if len(files) > 0 {
				shown := files
				extra := ""
				if len(shown) > 5 {
					extra = fmt.Sprintf(", ...and %d more", len(shown)-5)
					shown = shown[:5]
				}
				result.Uncommitted = append(result.Uncommitted,
					fmt.Sprintf("%s: %s%s", repoName, strings.Join(shown, ", "), extra))
			}
		}

		// Also check for recent stashes
		stashCmd := exec.CommandContext(ctx, "git", "log", "-g", "refs/stash", "--since="+sinceArg, "--format=%s")
		stashCmd.Dir = repoPath
		stashOut, err := stashCmd.Output()
		if err == nil {
			for _, line := range strings.Split(string(bytes.TrimSpace(stashOut)), "\n") {
				if line != "" {
					result.Stashed = append(result.Stashed, fmt.Sprintf("%s: %s", repoName, line))
				}
			}
		}
	}

	return result
}

// collectLocalMergedPRs finds GitHub-style merge commits authored by the user.
// These are omitted from the normal commit list (--no-merges) but are important
// standup signal when no GitHub token is configured.
func collectLocalMergedPRs(ctx context.Context, repoPath, repoName, sinceArg, authorNameCfg, authorEmailCfg string) []PRData {
	logCmd := exec.CommandContext(ctx, "git", "log", "--all", "--merges",
		"--since="+sinceArg,
		"--format=%aN%x00%aE%x00%s%x00%ci",
	)
	logCmd.Dir = repoPath
	logOut, err := logCmd.Output()
	if err != nil || len(bytes.TrimSpace(logOut)) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var out []PRData
	for _, line := range strings.Split(string(bytes.TrimSpace(logOut)), "\n") {
		parts := strings.SplitN(line, "\x00", 4)
		if len(parts) < 3 {
			continue
		}
		authorName := parts[0]
		authorEmail := parts[1]
		subject := parts[2]

		if !isLocalAuthor(authorName, authorEmail, authorNameCfg, authorEmailCfg) {
			continue
		}
		m := prMergeSubject.FindStringSubmatch(subject)
		if m == nil {
			continue
		}
		num, err := strconv.Atoi(m[1])
		if err != nil || num <= 0 {
			continue
		}
		key := fmt.Sprintf("%s#%d", repoName, num)
		if seen[key] {
			continue
		}
		seen[key] = true

		branchRef := m[2] // e.g. Tukesh1/slack-integration
		title := branchRef
		if i := strings.LastIndex(branchRef, "/"); i >= 0 && i+1 < len(branchRef) {
			title = strings.ReplaceAll(branchRef[i+1:], "-", " ")
		}

		out = append(out, PRData{
			Number:   num,
			Title:    title,
			RepoName: repoName,
			Type:     "merged",
		})
	}
	return out
}

func isLocalAuthor(authorName, authorEmail, authorNameCfg, authorEmailCfg string) bool {
	if authorNameCfg != "" && strings.EqualFold(authorName, authorNameCfg) {
		return true
	}
	if authorEmailCfg != "" && strings.EqualFold(authorEmail, authorEmailCfg) {
		return true
	}
	return false
}

package collector

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tukesh1/git-brief/internal/config"
)

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
func getGitIdent() (string, string) {
	name := config.Cfg.Author
	email := config.Cfg.Email

	if name == "" {
		if out, err := exec.Command("git", "config", "user.name").Output(); err == nil {
			name = strings.TrimSpace(string(out))
		}
	}
	if email == "" {
		if out, err := exec.Command("git", "config", "user.email").Output(); err == nil {
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
	Uncommitted []string // list of repos with uncommitted changes
	Stashed     []string // list of recent stashes
	Warnings    []Warning
	Repos       int
}

// CollectGitData reads commits from the configured/given workspaces.
func CollectGitData(ctx context.Context, since string, days int, workspaces []string) CollectResult {
	var result CollectResult

	authorNameCfg, authorEmailCfg := getGitIdent()
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

		// Check for uncommitted changes
		statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
		statusCmd.Dir = repoPath
		statusOut, err := statusCmd.Output()
		if err == nil {
			lines := strings.Split(string(bytes.TrimSpace(statusOut)), "\n")
			if len(lines) > 0 && lines[0] != "" {
				var files []string
				for i, line := range lines {
					if i >= 5 {
						files = append(files, fmt.Sprintf("...and %d more", len(lines)-5))
						break
					}
					if len(line) > 3 {
						files = append(files, line[3:]) // skip the " M " prefix
					}
				}
				result.Uncommitted = append(result.Uncommitted, fmt.Sprintf("%s: %s", repoName, strings.Join(files, ", ")))
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

package collector

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v62/github"
	"github.com/tukesh1/git-brief/internal/config"
)

// PRData holds information about a single GitHub pull request or issue.
type PRData struct {
	Type     string // "merged", "reviewed", "draft", or "issue"
	Title    string
	Number   int
	RepoName string // "owner/repo" format
}

// GithubResult holds the output of a GitHub data collection run.
type GithubResult struct {
	PRs      []PRData
	Warnings []Warning
}

// CollectGithubData fetches recently merged and reviewed PRs.
func CollectGithubData(ctx context.Context, since string, days int) GithubResult {
	var result GithubResult

	// GitHub is optional — skip quietly when not configured.
	if config.Cfg.GithubToken == "" || config.Cfg.GithubUsername == "" {
		return result
	}

	githubUser := config.Cfg.GithubUsername

	sinceTime := config.SinceTime(days)
	dateStr := sinceTime.Format("2006-01-02")

	client := github.NewClient(nil).WithAuthToken(config.Cfg.GithubToken)

	// Deduplicate by owner/repo#number so the same PR number in different
	// repositories is not collapsed incorrectly.
	seen := make(map[string]bool)

	appendIssue := func(issue *github.Issue, typ string) {
		repo := repoNameFromURL(issue.GetRepositoryURL())
		key := fmt.Sprintf("%s#%d", repo, issue.GetNumber())
		if seen[key] {
			return
		}
		seen[key] = true
		result.PRs = append(result.PRs, PRData{
			Type:     typ,
			Title:    issue.GetTitle(),
			Number:   issue.GetNumber(),
			RepoName: repo,
		})
	}

	// Merged PRs.
	mergedQuery := fmt.Sprintf("author:%s type:pr merged:>%s", githubUser, dateStr)
	mergedResult, _, err := client.Search.Issues(ctx, mergedQuery, &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 20},
	})
	if err == nil && mergedResult != nil {
		for _, issue := range mergedResult.Issues {
			appendIssue(issue, "merged")
		}
	} else if err != nil {
		result.Warnings = append(result.Warnings, Warning(fmt.Sprintf("GitHub merged-PR query failed: %v", err)))
	}

	// Reviewed PRs (skip if already counted as merged).
	reviewedQuery := fmt.Sprintf("reviewed-by:%s type:pr updated:>%s -author:%s", githubUser, dateStr, githubUser)
	reviewedResult, _, err := client.Search.Issues(ctx, reviewedQuery, &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 20},
	})
	if err == nil && reviewedResult != nil {
		for _, issue := range reviewedResult.Issues {
			appendIssue(issue, "reviewed")
		}
	} else if err != nil {
		result.Warnings = append(result.Warnings, Warning(fmt.Sprintf("GitHub reviewed-PR query failed: %v", err)))
	}

	// Draft/Unmerged PRs
	draftQuery := fmt.Sprintf("author:%s type:pr updated:>%s -is:merged", githubUser, dateStr)
	draftResult, _, err := client.Search.Issues(ctx, draftQuery, &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 20},
	})
	if err == nil && draftResult != nil {
		for _, issue := range draftResult.Issues {
			appendIssue(issue, "draft")
		}
	} else if err != nil {
		result.Warnings = append(result.Warnings, Warning(fmt.Sprintf("GitHub draft-PR query failed: %v", err)))
	}

	// Issue Activity
	issueQuery := fmt.Sprintf("involves:%s type:issue updated:>%s", githubUser, dateStr)
	issueResult, _, err := client.Search.Issues(ctx, issueQuery, &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 20},
	})
	if err == nil && issueResult != nil {
		for _, issue := range issueResult.Issues {
			appendIssue(issue, "issue")
		}
	} else if err != nil {
		result.Warnings = append(result.Warnings, Warning(fmt.Sprintf("GitHub issue query failed: %v", err)))
	}

	return result
}

// repoNameFromURL extracts "owner/repo" from a GitHub repository API URL.
func repoNameFromURL(url string) string {
	url = strings.TrimRight(url, "/")
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return url
}

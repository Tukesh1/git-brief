package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	anthropic "github.com/liushuangls/go-anthropic/v2"
	openai "github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"

	"github.com/tukesh1/git-brief/internal/collector"
	"github.com/tukesh1/git-brief/internal/config"
	"github.com/tukesh1/git-brief/internal/prompt"
)

const (
	aiTimeout = 60 * time.Second
	// maxCommitsInPrompt keeps the model focused when history is long.
	maxCommitsInPrompt = 25
	// When there is already solid commit history, compress local WIP so a
	// dirty tree does not drown out commits — but still include workspace signal.
	commitRichThreshold = 5
	maxSampleFiles      = 3
)

// SummarizeBrief builds a Slack standup from commits, PRs, and local workspace.
// It asks the LLM to theme-compress, then verifies the result still reflects
// the real data — if not, it falls back to a deterministic brief so the product
// goal (honest standup) is always achieved.
func SummarizeBrief(ctx context.Context, commits []collector.CommitData, prs []collector.PRData, uncommitted []string, stashed []string) (string, error) {
	now := time.Now()
	prepared := prepareCommits(commits)
	earlier, todayCommits := bucketCommits(prepared, now)
	promptString := buildUserPrompt(now, earlier, todayCommits, prs, uncommitted, stashed, len(prepared))

	provider := config.Cfg.LLMProvider
	var (
		brief string
		err   error
	)
	switch {
	case provider == "anthropic" && config.Cfg.AnthropicAPIKey != "":
		brief, err = callAnthropic(ctx, promptString)
	case provider == "openai" && config.Cfg.OpenAIAPIKey != "":
		brief, err = callOpenAI(ctx, promptString)
	case provider == "gemini" && config.Cfg.GeminiAPIKey != "":
		brief, err = callGemini(ctx, promptString)
	default:
		return "", fmt.Errorf("no valid LLM provider or API key configured (provider=%q)", provider)
	}
	if err != nil {
		// Still deliver a correct standup if the model is down.
		return buildDeterministicBrief(earlier, todayCommits, prs, uncommitted, stashed), nil
	}
	return ensureBriefMatchesData(brief, earlier, todayCommits, prs, uncommitted, stashed), nil
}

// buildUserPrompt pre-buckets activity the way an SDE thinks about standup:
// done before today vs done today vs local WIP still on disk.
func buildUserPrompt(
	now time.Time,
	earlier, todayCommits []collector.CommitData,
	prs []collector.PRData,
	uncommitted, stashed []string,
	commitCount int,
) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Today is %s, %s.\n", now.Format("Monday"), now.Format("January 2, 2006"))
	b.WriteString("Write a Slack standup from these pre-bucketed facts. Do not move items across buckets.\n")
	b.WriteString("CRITICAL: If COMPLETED BEFORE TODAY is non-empty, Yesterday MUST list those themes (2-4 bullets). Never leave Yesterday empty when that bucket has commits.\n\n")

	b.WriteString("COMPLETED BEFORE TODAY (→ Yesterday section):\n")
	writeCommitBucket(&b, earlier)

	b.WriteString("\nCOMPLETED TODAY (→ Today section):\n")
	writeCommitBucket(&b, todayCommits)

	b.WriteString("\nGITHUB PR/ISSUE ACTIVITY (merged/reviewed → Yesterday; drafts/issues → Today):\n")
	for _, line := range formatPRs(prs) {
		b.WriteString(line)
		b.WriteByte('\n')
	}

	b.WriteString(formatLocalWorkspace(uncommitted, stashed, commitCount))

	if len(earlier) == 0 && len(todayCommits) == 0 && len(prs) == 0 {
		b.WriteString("\nNOTE: No commits/PRs in the selected time range. Base the brief on LOCAL WORKSPACE WIP only. Yesterday: \"No shipped commits since last standup\". Do not invent shipped work.\n")
	}

	b.WriteString("\nWrite my standup brief now.")
	return b.String()
}

// prepareCommits sorts newest-first, drops pure noise when real commits remain,
// and caps length for the model.
func prepareCommits(commits []collector.CommitData) []collector.CommitData {
	if len(commits) == 0 {
		return commits
	}
	sorted := append([]collector.CommitData(nil), commits...)
	sort.SliceStable(sorted, func(i, j int) bool {
		ti, oi := parseCommitTime(sorted[i].Date)
		tj, oj := parseCommitTime(sorted[j].Date)
		if oi && oj {
			return ti.After(tj)
		}
		return i < j
	})

	filtered := filterNoiseCommits(sorted)
	if len(filtered) > maxCommitsInPrompt {
		filtered = filtered[:maxCommitsInPrompt]
	}
	return filtered
}

func filterNoiseCommits(commits []collector.CommitData) []collector.CommitData {
	var keep []collector.CommitData
	for _, c := range commits {
		if !isNoiseCommit(c.Message) {
			keep = append(keep, c)
		}
	}
	if len(keep) == 0 {
		return commits // only noise — keep it rather than empty the brief
	}
	return keep
}

func isNoiseCommit(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	m = strings.TrimPrefix(m, "chore:")
	m = strings.TrimPrefix(m, "chore(")
	m = strings.TrimSpace(m)
	switch m {
	case "", ".", "..", "wip", "tmp", "temp", "update", "updates", "fix", "minor", "misc":
		return true
	}
	noise := []string{
		"trigger ai review",
		"trigger ci",
		"bump version only",
		"empty commit",
	}
	for _, n := range noise {
		if strings.Contains(m, n) {
			return true
		}
	}
	return false
}

func writeCommitBucket(b *strings.Builder, commits []collector.CommitData) {
	if len(commits) == 0 {
		b.WriteString("  (none)\n")
		return
	}
	for _, c := range commits {
		fmt.Fprintf(b, "  [%s/%s] %s  (%s)\n", c.Repo, c.Branch, c.Message, c.Date)
	}
}

// bucketCommits splits commits into before-today vs today using committer dates.
func bucketCommits(commits []collector.CommitData, now time.Time) (earlier, today []collector.CommitData) {
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, c := range commits {
		t, ok := parseCommitTime(c.Date)
		if !ok {
			earlier = append(earlier, c)
			continue
		}
		if !t.Before(todayStart) {
			today = append(today, c)
		} else {
			earlier = append(earlier, c)
		}
	}
	return earlier, today
}

func parseCommitTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func formatPRs(prs []collector.PRData) []string {
	var prLines []string
	for _, p := range prs {
		action := "Reviewed"
		switch p.Type {
		case "merged":
			action = "Merged"
		case "draft":
			action = "Draft/WIP"
		case "issue":
			action = "Issue Activity"
		}
		if p.Type == "issue" {
			prLines = append(prLines, fmt.Sprintf("  %s #%d in %s: %s", action, p.Number, p.RepoName, p.Title))
		} else {
			prLines = append(prLines, fmt.Sprintf("  %s PR #%d in %s: %s", action, p.Number, p.RepoName, p.Title))
		}
	}
	if len(prLines) == 0 {
		prLines = append(prLines, "  (no GitHub PR activity found)")
	}
	return prLines
}

// formatLocalWorkspace always surfaces uncommitted/stash signal (core product
// goal), but compresses file lists when commit history is already rich so the
// model does not invent a story from a long dirty tree alone.
func formatLocalWorkspace(uncommitted, stashed []string, commitCount int) string {
	if len(uncommitted) == 0 && len(stashed) == 0 {
		return ""
	}

	var b strings.Builder
	if commitCount >= commitRichThreshold {
		b.WriteString("\nLOCAL WORKSPACE WIP (→ Today; one short bullet; do not invent beyond these paths):\n")
		for _, line := range uncommitted {
			b.WriteString("  ")
			b.WriteString(compactUncommittedLine(line))
			b.WriteByte('\n')
		}
	} else {
		b.WriteString("\nLOCAL WORKSPACE WIP (→ Today; primary signal when commits are empty/sparse):\n")
		for _, line := range uncommitted {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if len(stashed) > 0 {
		b.WriteString("STASHED WORK (→ Today):\n")
		for _, line := range stashed {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func compactUncommittedLine(line string) string {
	repo, filesPart, ok := strings.Cut(line, ": ")
	if !ok {
		return line
	}
	parts := splitFileList(filesPart)
	extra := 0
	var files []string
	for _, p := range parts {
		if strings.HasPrefix(p, "...and ") {
			var n int
			if _, err := fmt.Sscanf(p, "...and %d more", &n); err == nil {
				extra = n
			}
			continue
		}
		files = append(files, p)
	}
	total := len(files) + extra
	if total == 0 {
		return line
	}
	sample := files
	if len(sample) > maxSampleFiles {
		sample = sample[:maxSampleFiles]
	}
	return fmt.Sprintf("%s: ~%d dirty files (sample: %s)", repo, total, strings.Join(sample, ", "))
}

func splitFileList(s string) []string {
	raw := strings.Split(s, ", ")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// normalizeBrief strips common model wrappers so Slack paste stays clean.
func normalizeBrief(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```text")
	s = strings.TrimPrefix(s, "```markdown")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// Drop a one-line preamble before the first section header.
	for _, header := range []string{"Yesterday:", "Today:", "Blockers:"} {
		if i := strings.Index(s, header); i > 0 {
			s = strings.TrimSpace(s[i:])
			break
		}
	}
	return s
}

func float32Ptr(v float32) *float32 { return &v }

func callAnthropic(parent context.Context, promptString string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, aiTimeout)
	defer cancel()

	client := anthropic.NewClient(config.Cfg.AnthropicAPIKey)
	resp, err := client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model:       "claude-3-5-haiku-latest",
		MaxTokens:   300,
		Temperature: float32Ptr(0),
		System:      prompt.SystemPrompt,
		Messages: []anthropic.Message{
			{Role: "user", Content: []anthropic.MessageContent{anthropic.NewTextMessageContent(promptString)}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}
	if len(resp.Content) > 0 && resp.Content[0].Text != nil {
		return *resp.Content[0].Text, nil
	}
	return "", fmt.Errorf("anthropic: empty response")
}

func callOpenAI(parent context.Context, promptString string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, aiTimeout)
	defer cancel()

	client := openai.NewClient(config.Cfg.OpenAIAPIKey)
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       "gpt-4o-mini",
		Temperature: 0,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: prompt.SystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: promptString},
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai: empty response (no choices returned)")
	}
	return resp.Choices[0].Message.Content, nil
}

func callGemini(parent context.Context, promptString string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, aiTimeout)
	defer cancel()

	client, err := genai.NewClient(ctx, option.WithAPIKey(config.Cfg.GeminiAPIKey))
	if err != nil {
		return "", fmt.Errorf("gemini: create client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash")
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(prompt.SystemPrompt)},
	}
	temp := float32(0)
	model.Temperature = &temp

	resp, err := model.GenerateContent(ctx, genai.Text(promptString))
	if err != nil {
		return "", fmt.Errorf("gemini: %w", err)
	}

	if len(resp.Candidates) > 0 &&
		resp.Candidates[0].Content != nil &&
		len(resp.Candidates[0].Content.Parts) > 0 {
		if text, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
			return string(text), nil
		}
	}
	return "", fmt.Errorf("gemini: empty response")
}

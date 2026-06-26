package ai

import (
	"context"
	"fmt"
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

const aiTimeout = 60 * time.Second

// SummarizeBrief builds a prompt from commits and PRs, then calls the
// configured LLM provider to generate a standup brief.
func SummarizeBrief(commits []collector.CommitData, prs []collector.PRData, uncommitted []string, stashed []string) (string, error) {
	var commitLines []string
	for _, c := range commits {
		commitLines = append(commitLines, fmt.Sprintf("  [%s/%s] %s  (%s)", c.Repo, c.Branch, c.Message, c.Date))
	}
	if len(commitLines) == 0 {
		commitLines = append(commitLines, "  (no commits found in the time range)")
	}

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

	now := time.Now()

	uncommittedText := ""
	if len(uncommitted) > 0 {
		uncommittedText = "\nUNCOMMITTED CHANGES IN REPOS:\n" + strings.Join(uncommitted, "\n") + "\n"
	}

	stashedText := ""
	if len(stashed) > 0 {
		stashedText = "\nSTASHED WORK IN PROGRESS:\n" + strings.Join(stashed, "\n") + "\n"
	}

	promptString := fmt.Sprintf(`Today is %s, %s.

GIT COMMITS (%d total):
%s

GITHUB PR/ISSUE ACTIVITY (%d total):
%s
%s%s
Write my standup brief now.`,
		now.Format("Monday"),
		now.Format("January 2, 2006"),
		len(commits),
		strings.Join(commitLines, "\n"),
		len(prs),
		strings.Join(prLines, "\n"),
		uncommittedText,
		stashedText,
	)

	provider := config.Cfg.LLMProvider

	switch {
	case provider == "anthropic" && config.Cfg.AnthropicAPIKey != "":
		return callAnthropic(promptString)
	case provider == "openai" && config.Cfg.OpenAIAPIKey != "":
		return callOpenAI(promptString)
	case provider == "gemini" && config.Cfg.GeminiAPIKey != "":
		return callGemini(promptString)
	}

	return "", fmt.Errorf("no valid LLM provider or API key configured (provider=%q)", provider)
}

func callAnthropic(promptString string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), aiTimeout)
	defer cancel()

	client := anthropic.NewClient(config.Cfg.AnthropicAPIKey)
	resp, err := client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model:     "claude-3-5-haiku-latest",
		MaxTokens: 300,
		System:    prompt.SystemPrompt,
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

func callOpenAI(promptString string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), aiTimeout)
	defer cancel()

	client := openai.NewClient(config.Cfg.OpenAIAPIKey)
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "gpt-4o-mini",
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

func callGemini(promptString string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), aiTimeout)
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

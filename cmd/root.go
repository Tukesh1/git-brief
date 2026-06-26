package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tukesh/git-brief/internal/ai"
	"github.com/tukesh/git-brief/internal/collector"
	"github.com/tukesh/git-brief/internal/config"
	"github.com/tukesh/git-brief/internal/output"
)

var (
	sinceFlag     string
	daysFlag      int
	noClipboard   bool
	workspaceFlag []string
)

var dim = color.New(color.FgHiBlack)
var warn = color.New(color.FgYellow)

var rootCmd = &cobra.Command{
	Use:   "git-brief",
	Short: "AI-powered daily standup from your git log",
	Long: `git-brief reads your git commits and GitHub PR activity,
then generates a Slack-ready standup brief using AI.

Run 'git brief init' for first-time setup.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.InitConfig(); err != nil {
			return fmt.Errorf("config: %w", err)
		}

		if !hasAPIKey() {
			warn.Println("⚠️  No API key configured — launching setup wizard…")
			fmt.Println()
			if err := runInitWizard(); err != nil {
				return fmt.Errorf("setup: %w", err)
			}
			if err := config.InitConfig(); err != nil {
				return fmt.Errorf("config reload: %w", err)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		desc := config.SinceDescription(daysFlag)

		// ── Collect git commits ──────────────────────────────────────
		gitResult := collector.CollectGitData(ctx, sinceFlag, daysFlag, workspaceFlag)
		printWarnings(gitResult.Warnings)

		// ── Collect GitHub PRs ────────────────────────────────────────
		ghResult := collector.CollectGithubData(ctx, sinceFlag, daysFlag)
		printWarnings(ghResult.Warnings)

		// ── Generate brief ───────────────────────────────────────────
		if len(gitResult.Commits) == 0 && len(ghResult.PRs) == 0 && len(gitResult.Uncommitted) == 0 {
			warn.Println("\n⚠️  No commits, PRs, or uncommitted changes found. Nothing to summarise.")
			warn.Printf("   Looked %s in %d repo(s).\n", desc, gitResult.Repos)
			warn.Println("   Try: git brief --days 3")
			return nil
		}

		brief, err := ai.SummarizeBrief(gitResult.Commits, ghResult.PRs, gitResult.Uncommitted, gitResult.Stashed)
		if err != nil {
			return fmt.Errorf("AI: %w", err)
		}

		output.PrintBrief(brief)
		if !noClipboard {
			output.CopyToClipboard(brief)
		}
		return nil
	},
}

func hasAPIKey() bool {
	switch config.Cfg.LLMProvider {
	case "anthropic":
		return config.Cfg.AnthropicAPIKey != ""
	case "gemini":
		return config.Cfg.GeminiAPIKey != ""
	case "openai":
		return config.Cfg.OpenAIAPIKey != ""
	}
	return false
}

func printWarnings(warnings []collector.Warning) {
	for _, w := range warnings {
		warn.Printf("  ⚠️  %s\n", w)
	}
}

// Execute is the binary entry point called from main.go.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVar(&sinceFlag, "since", "", `Override time range (e.g. "monday", "2 days ago")`)
	rootCmd.Flags().IntVar(&daysFlag, "days", 0, "Look back N days instead of yesterday/last-Friday")
	rootCmd.Flags().BoolVar(&noClipboard, "no-clipboard", false, "Print the brief but skip clipboard copy")
	rootCmd.Flags().StringSliceVarP(&workspaceFlag, "workspace", "w", []string{}, "Override workspace directories")
}

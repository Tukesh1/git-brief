package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/tukesh1/git-brief/internal/ai"
	"github.com/tukesh1/git-brief/internal/collector"
	"github.com/tukesh1/git-brief/internal/config"
	"github.com/tukesh1/git-brief/internal/output"
	"github.com/tukesh1/git-brief/internal/slack"
)

var (
	sinceFlag     string
	daysFlag      int
	noClipboard   bool
	slackFlag     bool
	noSlackFlag   bool
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

		maybePostToSlack(ctx, brief)
		return nil
	},
}

// maybePostToSlack performs the manual-approval Slack hand-off. It never posts
// to Slack itself: it copies the brief to the clipboard and opens the target
// channel in the user's Slack client, where they paste it and press send. The
// message is therefore always posted as the user (never as a bot) and only
// after they explicitly confirm twice — once here, and again in Slack.
func maybePostToSlack(ctx context.Context, brief string) {
	if noSlackFlag {
		return
	}

	channel := config.Cfg.SlackChannel
	if channel == "" {
		if slackFlag {
			warn.Println("\n⚠️  --slack given but no Slack channel configured. Run `git brief init`.")
		}
		return
	}

	// Manual approval gate #1: decide whether to open Slack at all.
	//   --slack    → always open (non-interactive friendly)
	//   --no-slack → handled above (never open)
	//   otherwise  → prompt, but only when attached to a real terminal so we
	//                never hang a scripted/CI run.
	if !slackFlag {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return
		}
		open := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Open Slack %s to post this brief? (you press send yourself)", channel),
			Default: true,
		}
		if err := survey.AskOne(prompt, &open); err != nil || !open {
			return
		}
	}

	// Decide what to open. The token is entirely optional and read-only — it is
	// only used to turn a #name into a channel ID. Employees who are not
	// workspace admins (and so cannot install an app to get a token) can simply
	// paste a channel link (Slack ▸ channel ▸ Copy link) or a channel ID.
	var openTarget, webLink string

	if _, _, ok := slack.ParseChannelURL(channel); ok {
		// User pasted a Slack link copied from the app: open it as-is. It needs
		// no token and already routes to the correct workspace + channel.
		openTarget = channel
		if cid, tid, _ := slack.ParseChannelURL(channel); cid != "" {
			_, webLink = slack.ChannelLinks(cid, tid)
		} else {
			webLink = channel
		}
	} else {
		channelID := channel
		teamID := ""
		if token := config.Cfg.SlackToken; token != "" {
			client := slack.NewClient(token)
			if id, err := client.ResolveChannel(ctx, channel); err == nil {
				channelID = id
			} else {
				warn.Printf("  ⚠️  could not resolve %q: %v\n", channel, err)
			}
			if info, err := client.AuthTest(ctx); err == nil {
				teamID = info.TeamID
			}
		} else if !slack.IsChannelID(channelID) {
			warn.Println("  ⚠️  No Slack token set and the channel is a name.")
			warn.Println("     Tip: in Slack, open the channel ▸ click its name ▸ Copy link, then paste")
			warn.Println("     that into slack_channel (or use a channel ID like C0123ABCD).")
			warn.Println("     No token or workspace-admin rights are needed for this.")
		}
		openTarget, webLink = slack.ChannelLinks(channelID, teamID)
	}

	// The brief must be on the clipboard so the user can paste it, even when
	// --no-clipboard was passed for stdout.
	if noClipboard {
		output.CopyToClipboard(brief)
	}

	fmt.Println()
	dim.Println("  Opening Slack — your brief is on the clipboard.")
	fmt.Printf("  Channel: %s\n", channel)
	fmt.Printf("  Link:    %s\n", webLink)
	if err := slack.OpenURL(ctx, openTarget); err != nil {
		warn.Printf("  ⚠️  couldn't open Slack automatically: %v\n", err)
		fmt.Printf("  Open this link manually: %s\n", openTarget)
	}
	fmt.Println()
	color.New(color.Bold).Println("  ➜ Paste (Cmd/Ctrl+V) into the channel, then press Enter to post as yourself.")
	dim.Println("    Nothing is sent automatically — you have the final say in Slack.")
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
	rootCmd.Flags().BoolVar(&slackFlag, "slack", false, "Open the configured Slack channel to post the brief (no prompt)")
	rootCmd.Flags().BoolVar(&noSlackFlag, "no-slack", false, "Never open Slack, even if a channel is configured")
	rootCmd.Flags().StringSliceVarP(&workspaceFlag, "workspace", "w", []string{}, "Override workspace directories")
}

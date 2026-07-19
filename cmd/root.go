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
	slackOpenFlag bool
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
		if sinceFlag != "" {
			desc = "since " + sinceFlag
		}

		gitResult := collector.CollectGitData(ctx, sinceFlag, daysFlag, workspaceFlag)
		printWarnings(gitResult.Warnings)

		ghResult := collector.CollectGithubData(ctx, sinceFlag, daysFlag)
		printWarnings(ghResult.Warnings)

		// Prefer GitHub API PRs when available; always include local merge commits
		// so merged PRs show up even without a GitHub token.
		prs := mergePRLists(ghResult.PRs, gitResult.MergedPRs)

		if len(gitResult.Commits) == 0 && len(prs) == 0 && len(gitResult.Uncommitted) == 0 && len(gitResult.Stashed) == 0 {
			warn.Printf("Nothing to summarise %s (%d repo(s)). Try --days 7\n", desc, gitResult.Repos)
			return nil
		}

		brief, err := ai.SummarizeBrief(ctx, gitResult.Commits, prs, gitResult.Uncommitted, gitResult.Stashed)
		if err != nil {
			return fmt.Errorf("AI: %w", err)
		}

		output.PrintBrief(brief)

		// Clipboard + Slack get Slack mrkdwn headers; terminal stays plain for colouring.
		slackBrief := output.ForSlack(brief)
		if !noClipboard {
			output.CopyToClipboard(slackBrief)
		}

		maybePostToSlack(ctx, slackBrief)
		return nil
	},
}

// maybePostToSlack delivers the brief to Slack after the user approves.
//
// Two modes:
//   - Background send (default when a Slack token is configured): posts the
//     brief straight to the channel via the Web API, as the user, with no Slack
//     window opening and no manual paste. Requires a chat:write user token.
//   - Open hand-off (no token, or --slack-open): copies the brief to the
//     clipboard and opens the channel in Slack so the user pastes and sends.
//
// Either way git-brief never sends without approval: an interactive confirm
// gates the action (skipped with --slack for scripting), and the open hand-off
// always leaves the final send to the user inside Slack.
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

	token := config.Cfg.SlackToken
	// Background sending needs a write-capable token; otherwise we open Slack.
	background := token != "" && !slackOpenFlag

	// Approval gate. --slack opts in non-interactively; otherwise we proceed
	// only after an interactive confirmation and never run unattended.
	if !slackFlag {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return
		}
		msg := fmt.Sprintf("Open Slack %s to post this brief? (you press send)", channel)
		if background {
			msg = fmt.Sprintf("Post this brief to Slack %s now, as you? (sent immediately)", channel)
		}
		ok := false
		if err := survey.AskOne(&survey.Confirm{Message: msg, Default: true}, &ok); err != nil || !ok {
			return
		}
	}

	if background {
		if postBriefToSlack(ctx, brief, channel, token) {
			return
		}
		dim.Println("Post failed — opening Slack to paste instead…")
	}
	openSlackHandoff(ctx, brief, channel, token)
}

// postBriefToSlack posts the brief to the channel via the Slack Web API, as the
// authenticated user, with no window opening. It returns false (so the caller
// can fall back to the open hand-off) when the channel can't be resolved or the
// API rejects the post — e.g. the token lacks the chat:write scope.
func postBriefToSlack(ctx context.Context, brief, channel, token string) bool {
	client := slack.NewClient(token)

	channelID, ok := resolveChannelForPost(ctx, client, channel)
	if !ok {
		return false
	}

	ts, err := client.PostMessage(ctx, channelID, brief)
	if err != nil {
		warn.Printf("  ⚠️  couldn't post to Slack: %v\n", err)
		return false
	}

	fmt.Println()
	color.New(color.FgCyan).Printf("✅ Posted to Slack\n")
	if link, err := client.GetPermalink(ctx, channelID, ts); err == nil && link != "" {
		dim.Println(link)
	}
	return true
}

// resolveChannelForPost turns a channel link / ID / #name into a channel ID for
// chat.postMessage.
func resolveChannelForPost(ctx context.Context, client *slack.Client, channel string) (string, bool) {
	if cid, _, isURL := slack.ParseChannelURL(channel); isURL {
		if cid != "" {
			return cid, true
		}
		warn.Printf("  ⚠️  couldn't read a channel ID from %q\n", channel)
		return "", false
	}
	if slack.IsChannelID(channel) {
		return channel, true
	}
	id, err := client.ResolveChannel(ctx, channel)
	if err != nil {
		warn.Printf("  ⚠️  could not resolve %q: %v\n", channel, err)
		return "", false
	}
	return id, true
}

// openSlackHandoff copies the brief to the clipboard and opens the channel in
// the user's Slack client so they paste and send manually. Needs no token: a
// pasted channel link or ID works for anyone, no workspace-admin rights needed.
func openSlackHandoff(ctx context.Context, brief, channel, token string) {
	var openTarget, webLink string

	if _, _, isURL := slack.ParseChannelURL(channel); isURL {
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
		if token != "" {
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
	if err := slack.OpenURL(ctx, openTarget); err != nil {
		warn.Printf("Couldn't open Slack: %v\nOpen: %s\n", err, openTarget)
	} else {
		dim.Printf("Opened Slack — paste from clipboard (%s)\n", webLink)
	}
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

// mergePRLists prefers API results, then fills gaps from local merge commits.
func mergePRLists(api, local []collector.PRData) []collector.PRData {
	if len(local) == 0 {
		return api
	}
	seen := make(map[string]bool, len(api)+len(local))
	out := make([]collector.PRData, 0, len(api)+len(local))
	for _, p := range api {
		key := fmt.Sprintf("%s#%d", p.RepoName, p.Number)
		seen[key] = true
		out = append(out, p)
	}
	for _, p := range local {
		key := fmt.Sprintf("%s#%d", p.RepoName, p.Number)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}

// Execute is the binary entry point called from main.go.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVar(&sinceFlag, "since", "", `Override time range (e.g. "monday", "2 days ago")`)
	rootCmd.Flags().IntVar(&daysFlag, "days", 0, "Look back N days instead of yesterday/last-Friday")
	rootCmd.Flags().BoolVar(&noClipboard, "no-clipboard", false, "Print the brief but skip clipboard copy")
	rootCmd.Flags().BoolVar(&slackFlag, "slack", false, "Send the brief to the configured Slack channel without prompting")
	rootCmd.Flags().BoolVar(&noSlackFlag, "no-slack", false, "Never touch Slack, even if a channel is configured")
	rootCmd.Flags().BoolVar(&slackOpenFlag, "slack-open", false, "Open Slack to paste/send manually instead of background posting")
	rootCmd.Flags().StringSliceVarP(&workspaceFlag, "workspace", "w", []string{}, "Override workspace directories")
}

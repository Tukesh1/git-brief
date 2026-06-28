package cmd

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tukesh1/git-brief/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive setup wizard",
	Long:  "Walk through the one-time configuration of workspaces, LLM provider, and API keys.",
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = config.InitConfig()
		if err := runInitWizard(); err != nil {
			return err
		}
		fmt.Println()
		color.Green("Run `git brief` to generate your first brief.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInitWizard() error {
	fmt.Println()
	color.New(color.Bold).Println("  Welcome to git-brief setup!")
	fmt.Println()

	// ── Workspaces ─────────────────────────────────────────────
	var workspacesStr string
	if err := survey.AskOne(&survey.Input{
		Message: "Workspace directories (comma-separated; blank = use cwd):",
		Default: strings.Join(config.Cfg.Workspaces, ", "),
	}, &workspacesStr); err != nil {
		return fmt.Errorf("setup cancelled")
	}

	if s := strings.TrimSpace(workspacesStr); s != "" {
		var w []string
		for _, p := range strings.Split(s, ",") {
			if t := strings.TrimSpace(p); t != "" {
				w = append(w, t)
			}
		}
		config.Cfg.Workspaces = w
	} else {
		config.Cfg.Workspaces = []string{}
	}

	// ── LLM provider ───────────────────────────────────────────
	var providerChoice string
	if err := survey.AskOne(&survey.Select{
		Message: "LLM provider:",
		Options: []string{"Google (Gemini) — free tier available", "Anthropic (Claude)", "OpenAI (GPT)"},
		Default: "Google (Gemini) — free tier available",
	}, &providerChoice); err != nil {
		return fmt.Errorf("setup cancelled")
	}

	switch {
	case strings.Contains(providerChoice, "Anthropic"):
		config.Cfg.LLMProvider = "anthropic"
		if err := askSecret("Anthropic API Key:", &config.Cfg.AnthropicAPIKey); err != nil {
			return err
		}
	case strings.Contains(providerChoice, "Gemini"):
		config.Cfg.LLMProvider = "gemini"
		if err := askSecret("Gemini API Key:", &config.Cfg.GeminiAPIKey); err != nil {
			return err
		}
	case strings.Contains(providerChoice, "OpenAI"):
		config.Cfg.LLMProvider = "openai"
		if err := askSecret("OpenAI API Key:", &config.Cfg.OpenAIAPIKey); err != nil {
			return err
		}
	}

	// ── GitHub (optional) ──────────────────────────────────────
	var useGithub bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "Enable GitHub PR integration? (requires a personal access token)",
		Default: config.Cfg.GithubToken != "",
	}, &useGithub)

	if useGithub {
		if err := askSecret("GitHub Personal Access Token:", &config.Cfg.GithubToken); err != nil {
			return err
		}
		if err := survey.AskOne(&survey.Input{
			Message: "GitHub username:",
			Default: config.Cfg.GithubUsername,
		}, &config.Cfg.GithubUsername); err != nil {
			return fmt.Errorf("setup cancelled")
		}
	}

	// ── Slack (optional) ───────────────────────────────────────
	// git-brief never posts to Slack itself: it copies the brief to your
	// clipboard and opens the channel so you paste and press send yourself.
	// That means no bot, no write scope, and no workspace-admin rights needed.
	var useSlack bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "Enable Slack hand-off? (opens the channel so you can post the brief yourself)",
		Default: config.Cfg.SlackChannel != "",
	}, &useSlack)

	if useSlack {
		fmt.Println("  Tip: in Slack, open the channel ▸ click its name ▸ Copy link, and paste it below.")
		if err := survey.AskOne(&survey.Input{
			Message: "Slack channel (paste a channel link, or #name, or a channel ID like C0123ABCD):",
			Default: config.Cfg.SlackChannel,
		}, &config.Cfg.SlackChannel); err != nil {
			return fmt.Errorf("setup cancelled")
		}
		// The token is OPTIONAL. With a chat:write user token (xoxp-…) git-brief
		// can post the brief in the background as you (no window, no pasting).
		// Without one it opens the channel so you paste and send manually — which
		// needs no token and no workspace-admin rights.
		fmt.Println("  Optional: an xoxp- user token with chat:write lets git-brief post in the background as you.")
		fmt.Println("  Leave it blank to instead open Slack and paste/send manually (no token needed).")
		if err := askSecret("Slack user token (optional; press Enter to skip):", &config.Cfg.SlackToken); err != nil {
			return err
		}
	} else {
		// Clear them if they disabled it
		config.Cfg.SlackToken = ""
		config.Cfg.SlackChannel = ""
	}

	// ── Persist ────────────────────────────────────────────────
	if err := config.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	configPath, _ := config.ConfigPath()
	fmt.Println()
	color.Green("✅ Setup complete!")
	color.New(color.FgHiBlack).Printf("   Config saved to %s\n", configPath)
	return nil
}

func askSecret(message string, dest *string) error {
	if err := survey.AskOne(&survey.Password{Message: message}, dest); err != nil {
		return fmt.Errorf("setup cancelled")
	}
	return nil
}

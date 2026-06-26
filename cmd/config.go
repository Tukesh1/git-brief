package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tukesh/git-brief/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show the path and contents of the configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ConfigPath()
		if err != nil {
			return fmt.Errorf("could not resolve config path: %w", err)
		}

		color.New(color.Bold).Println("Configuration file:")
		color.Green(path)
		fmt.Println()

		// Load the current config to display it.
		if err := config.InitConfig(); err != nil {
			color.Yellow("(config file not found or unreadable — run `git brief init` to create it)")
			return nil
		}

		// Display a masked view so keys are never shown in plaintext.
		type display struct {
			Author          string   `json:"author"`
			Email           string   `json:"email"`
			Workspaces      []string `json:"workspaces"`
			GithubUsername  string   `json:"github_username"`
			GithubToken     string   `json:"github_token"`
			LLMProvider     string   `json:"llm_provider"`
			AnthropicAPIKey string   `json:"anthropic_api_key"`
			GeminiAPIKey    string   `json:"gemini_api_key"`
			OpenAIAPIKey    string   `json:"openai_api_key"`
		}

		cfg := config.Cfg
		d := display{
			Author:          cfg.Author,
			Email:           cfg.Email,
			Workspaces:      cfg.Workspaces,
			GithubUsername:  cfg.GithubUsername,
			GithubToken:     maskKey(cfg.GithubToken),
			LLMProvider:     cfg.LLMProvider,
			AnthropicAPIKey: maskKey(cfg.AnthropicAPIKey),
			GeminiAPIKey:    maskKey(cfg.GeminiAPIKey),
			OpenAIAPIKey:    maskKey(cfg.OpenAIAPIKey),
		}

		out, _ := json.MarshalIndent(d, "", "  ")
		fmt.Println(string(out))
		fmt.Println()
		fmt.Println("You can edit this file directly or run `git brief init` to reconfigure.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

// maskKey masks an API key leaving only the first 8 characters visible.
//
//	"" -- "(not set)"
//	short keys -- "****"
//	normal keys -- "sk-ant-ap****"
func maskKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:8] + "****"
}

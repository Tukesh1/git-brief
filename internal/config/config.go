package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Config holds all user-configurable settings persisted to disk.
type Config struct {
	Author          string   `mapstructure:"author"           json:"author"`
	Email           string   `mapstructure:"email"            json:"email"`
	Workspaces      []string `mapstructure:"workspaces"       json:"workspaces"`
	GithubToken     string   `mapstructure:"github_token"     json:"github_token"`
	GithubUsername  string   `mapstructure:"github_username"  json:"github_username"`
	LLMProvider     string   `mapstructure:"llm_provider"     json:"llm_provider"`
	AnthropicAPIKey string   `mapstructure:"anthropic_api_key" json:"anthropic_api_key"`
	GeminiAPIKey    string   `mapstructure:"gemini_api_key"   json:"gemini_api_key"`
	OpenAIAPIKey    string   `mapstructure:"openai_api_key"   json:"openai_api_key"`
}

// Cfg is the global configuration instance, populated by InitConfig.
var Cfg Config

// ConfigPath returns the absolute path to the JSON config file.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "git-brief", "config.json"), nil
}

// weekdayDaysBack returns how many calendar days to go back based on the
// current weekday so that weekends are skipped automatically:
//
//	Monday or Sunday → 3 (back to Friday)
//	Saturday         → 1 (back to Friday)
//	Tue–Fri          → 1 (yesterday)
func weekdayDaysBack(wd time.Weekday) int {
	switch wd {
	case time.Monday, time.Sunday:
		return 3
	case time.Saturday:
		return 1
	default:
		return 1
	}
}

// SinceTime computes the "since" boundary as a time.Time.
//   - days > 0 → exactly N calendar days ago at 00:00
//   - days == 0 → smart weekday detection (skips weekends)
func SinceTime(days int) time.Time {
	now := time.Now()
	dayAt := func(daysBack int) time.Time {
		d := now.AddDate(0, 0, -daysBack)
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
	}

	if days > 0 {
		return dayAt(days)
	}
	return dayAt(weekdayDaysBack(now.Weekday()))
}

// SinceDescription returns a human-friendly label like "since yesterday"
// or "for the last 3 days" for use in progress output.
func SinceDescription(days int) string {
	if days > 0 {
		if days == 1 {
			return "since yesterday"
		}
		return fmt.Sprintf("for the last %d days", days)
	}
	wd := time.Now().Weekday()
	switch wd {
	case time.Monday, time.Sunday:
		return "since last Friday"
	default:
		return "since yesterday"
	}
}

// InitConfig initialises viper and reads the config file into Cfg.
func InitConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(home, ".config", "git-brief")
	if err = os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath(configDir)

	viper.SetDefault("workspaces", []string{})
	viper.SetDefault("llm_provider", "gemini")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	Cfg = Config{}
	return viper.Unmarshal(&Cfg)
}

// SaveConfig serialises the current Cfg struct to disk as formatted JSON.
func SaveConfig() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(&Cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

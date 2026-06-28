package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/tukesh1/git-brief/internal/config"
)

// captureOutput redirects everything maybePostToSlack writes (both fmt.* to
// os.Stdout and fatih/color writes to color.Output) into a buffer.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	origStdout := os.Stdout
	origColorOut := color.Output
	origNoColor := color.NoColor
	os.Stdout = w
	color.Output = w
	color.NoColor = true
	defer func() {
		os.Stdout = origStdout
		color.Output = origColorOut
		color.NoColor = origNoColor
	}()

	fn()
	_ = w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

// TestMaybePostToSlackHandoff drives the real CLI hand-off against a mock Slack
// server: it resolves a #name to an ID, looks up the team, and prints the
// channel link the user is sent to. No real brief or LLM key is required.
func TestMaybePostToSlackHandoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":                true,
				"channels":          []map[string]string{{"id": "C0ANNOUNCE", "name": "announcements"}},
				"response_metadata": map[string]string{"next_cursor": ""},
			})
		case "/auth.test":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "team_id": "T0WORKSPACE"})
		default:
			t.Errorf("unexpected Slack path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("SLACK_API_BASE", srv.URL)

	// Configure Slack and force the hand-off without an interactive prompt.
	config.Cfg = config.Config{SlackToken: "xoxp-test", SlackChannel: "#announcements"}
	slackFlag = true
	noSlackFlag = false
	noClipboard = false
	t.Cleanup(func() { slackFlag = false; config.Cfg = config.Config{} })

	out := captureOutput(t, func() {
		maybePostToSlack(context.Background(), "Yesterday:\n• shipped feature\n\nToday:\n• write docs")
	})

	t.Logf("rendered hand-off output:\n%s", out)

	for _, want := range []string{
		"Opening Slack",
		"#announcements",
		"https://slack.com/app_redirect?channel=C0ANNOUNCE", // proves #name resolved to ID via the API
		"Paste", // manual-send instruction
		"final say in Slack",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("hand-off output missing %q.\n--- output ---\n%s", want, out)
		}
	}
}

// TestMaybePostToSlackDisabled confirms --no-slack suppresses the hand-off.
func TestMaybePostToSlackDisabled(t *testing.T) {
	config.Cfg = config.Config{SlackChannel: "#announcements"}
	noSlackFlag = true
	t.Cleanup(func() { noSlackFlag = false; config.Cfg = config.Config{} })

	out := captureOutput(t, func() {
		maybePostToSlack(context.Background(), "brief")
	})
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no output with --no-slack, got: %q", out)
	}
}

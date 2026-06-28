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

// TestMaybePostToSlackBackgroundSend drives the default token path: it resolves
// a #name, posts via chat.postMessage as the user (no window), and prints the
// permalink — all against a mock Slack server. No real brief or LLM key needed.
func TestMaybePostToSlackBackgroundSend(t *testing.T) {
	var posted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":                true,
				"channels":          []map[string]string{{"id": "C0ANNOUNCE", "name": "announcements"}},
				"response_metadata": map[string]string{"next_cursor": ""},
			})
		case "/chat.postMessage":
			posted = true
			if r.Method != http.MethodPost {
				t.Errorf("chat.postMessage method = %s, want POST", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "ts": "1700000000.000100"})
		case "/chat.getPermalink":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":        true,
				"permalink": "https://acme.slack.com/archives/C0ANNOUNCE/p1700000000000100",
			})
		default:
			t.Errorf("unexpected Slack path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("SLACK_API_BASE", srv.URL)

	config.Cfg = config.Config{SlackToken: "xoxp-test", SlackChannel: "#announcements"}
	slackFlag = true
	noSlackFlag = false
	slackOpenFlag = false
	noClipboard = false
	t.Cleanup(func() { slackFlag = false; config.Cfg = config.Config{} })

	out := captureOutput(t, func() {
		maybePostToSlack(context.Background(), "Yesterday:\n• shipped feature\n\nToday:\n• write docs")
	})
	t.Logf("rendered background-send output:\n%s", out)

	if !posted {
		t.Error("expected chat.postMessage to be called in background mode")
	}
	for _, want := range []string{
		"Posted to Slack",
		"no window needed",
		"https://acme.slack.com/archives/C0ANNOUNCE/p1700000000000100", // permalink
	} {
		if !strings.Contains(out, want) {
			t.Errorf("background-send output missing %q.\n--- output ---\n%s", want, out)
		}
	}
	if strings.Contains(out, "Opening Slack") {
		t.Errorf("background mode must NOT open Slack.\n--- output ---\n%s", out)
	}
}

// TestMaybePostToSlackBackgroundFallback verifies that when the token lacks
// chat:write, git-brief falls back to opening Slack for a manual send.
func TestMaybePostToSlackBackgroundFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":                true,
				"channels":          []map[string]string{{"id": "C0ANNOUNCE", "name": "announcements"}},
				"response_metadata": map[string]string{"next_cursor": ""},
			})
		case "/chat.postMessage":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "missing_scope"})
		case "/auth.test":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "team_id": "T0WORKSPACE"})
		default:
			t.Errorf("unexpected Slack path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("SLACK_API_BASE", srv.URL)

	config.Cfg = config.Config{SlackToken: "xoxp-test", SlackChannel: "#announcements"}
	slackFlag = true
	noSlackFlag = false
	slackOpenFlag = false
	noClipboard = false
	t.Cleanup(func() { slackFlag = false; config.Cfg = config.Config{} })

	out := captureOutput(t, func() {
		maybePostToSlack(context.Background(), "brief body")
	})
	t.Logf("rendered fallback output:\n%s", out)

	for _, want := range []string{"missing_scope", "falling back", "Opening Slack"} {
		if !strings.Contains(out, want) {
			t.Errorf("fallback output missing %q.\n--- output ---\n%s", want, out)
		}
	}
}

// TestMaybePostToSlackOpenFlag verifies --slack-open forces the manual hand-off
// even when a token is configured (resolving the channel for a native link).
func TestMaybePostToSlackOpenFlag(t *testing.T) {
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
		case "/chat.postMessage":
			t.Error("--slack-open must NOT post via the API")
		default:
			t.Errorf("unexpected Slack path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("SLACK_API_BASE", srv.URL)

	config.Cfg = config.Config{SlackToken: "xoxp-test", SlackChannel: "#announcements"}
	slackFlag = true
	slackOpenFlag = true
	noSlackFlag = false
	noClipboard = false
	t.Cleanup(func() { slackFlag = false; slackOpenFlag = false; config.Cfg = config.Config{} })

	out := captureOutput(t, func() {
		maybePostToSlack(context.Background(), "brief body")
	})
	for _, want := range []string{"Opening Slack", "https://slack.com/app_redirect?channel=C0ANNOUNCE", "Paste"} {
		if !strings.Contains(out, want) {
			t.Errorf("--slack-open output missing %q.\n--- output ---\n%s", want, out)
		}
	}
}

// TestMaybePostToSlackNoTokenChannelLink proves the non-admin path: an employee
// pastes a channel link (Slack ▸ Copy link) with NO token, and the hand-off
// works without making any Slack API call.
func TestMaybePostToSlackNoTokenChannelLink(t *testing.T) {
	// Any HTTP call would mean we wrongly required a token.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected Slack API call to %s — no token path must not hit the API", r.URL.Path)
	}))
	defer srv.Close()
	t.Setenv("SLACK_API_BASE", srv.URL)

	config.Cfg = config.Config{SlackChannel: "https://acme.slack.com/archives/C0ANNOUNCE"}
	slackFlag = true
	noSlackFlag = false
	noClipboard = false
	t.Cleanup(func() { slackFlag = false; config.Cfg = config.Config{} })

	out := captureOutput(t, func() {
		maybePostToSlack(context.Background(), "Today:\n• ship it")
	})
	t.Logf("rendered hand-off output (no token):\n%s", out)

	for _, want := range []string{
		"Opening Slack",
		"https://slack.com/app_redirect?channel=C0ANNOUNCE", // CID parsed from the pasted link
		"Paste",
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

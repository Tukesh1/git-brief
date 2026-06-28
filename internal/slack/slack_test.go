package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsChannelID(t *testing.T) {
	cases := map[string]bool{
		"C0123ABCD":   true,
		"G0123ABCD":   true,
		"D0123ABCD":   true,
		"C12345678":   true,
		"#standups":   false,
		"standups":    false,
		"team-eng":    false,
		"":            false,
		"c0123abcd":   false, // lowercase is a name, not an ID
		"C123":        false, // too short
		"XSOMETHING1": false, // wrong prefix
	}
	for in, want := range cases {
		if got := IsChannelID(in); got != want {
			t.Errorf("IsChannelID(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestChannelLinks(t *testing.T) {
	// With a team ID we get a native deep link plus the web fallback.
	deep, web := ChannelLinks("C0123ABCD", "T0001")
	if deep != "slack://channel?team=T0001&id=C0123ABCD" {
		t.Errorf("deep link = %q", deep)
	}
	if web != "https://slack.com/app_redirect?channel=C0123ABCD" {
		t.Errorf("web link = %q", web)
	}

	// Without a team ID both links fall back to the web redirect.
	deep, web = ChannelLinks("C0123ABCD", "")
	if deep != web || web != "https://slack.com/app_redirect?channel=C0123ABCD" {
		t.Errorf("fallback links = %q / %q", deep, web)
	}
}

func TestParseChannelURL(t *testing.T) {
	cases := []struct {
		in       string
		wantID   string
		wantTeam string
		wantOK   bool
	}{
		// "Copy link" on a channel — the non-admin, no-token path.
		{"https://acme.slack.com/archives/C0123ABCD", "C0123ABCD", "", true},
		{"https://acme.slack.com/archives/C0123ABCD/p1700000000000100", "C0123ABCD", "", true},
		// In-app client URL carries both team and channel IDs.
		{"https://app.slack.com/client/T0AAAA11/C0123ABCD", "C0123ABCD", "T0AAAA11", true},
		// Native deep link.
		{"slack://channel?team=T0AAAA11&id=C0123ABCD", "C0123ABCD", "T0AAAA11", true},
		// app_redirect web link.
		{"https://slack.com/app_redirect?channel=C0123ABCD", "C0123ABCD", "", true},
		// Not URLs — handled by the name/ID path instead.
		{"#standups", "", "", false},
		{"standups", "", "", false},
		{"C0123ABCD", "", "", false},
		// A non-Slack URL must not be treated as a channel link.
		{"https://example.com/archives/C0123ABCD", "", "", false},
	}
	for _, c := range cases {
		id, team, ok := ParseChannelURL(c.in)
		if ok != c.wantOK || id != c.wantID || team != c.wantTeam {
			t.Errorf("ParseChannelURL(%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.in, id, team, ok, c.wantID, c.wantTeam, c.wantOK)
		}
	}
}

// mockSlack spins up an httptest server that mimics the two Slack endpoints we
// call, and points the package at it via SLACK_API_BASE.
func mockSlack(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Setenv("SLACK_API_BASE", srv.URL)
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveChannelAlreadyID(t *testing.T) {
	// A bare ID must short-circuit without any HTTP call.
	mockSlack(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected HTTP call to %s", r.URL.Path)
	})
	got, err := NewClient("xoxp-test").ResolveChannel(context.Background(), "C0123ABCD")
	if err != nil {
		t.Fatalf("ResolveChannel: %v", err)
	}
	if got != "C0123ABCD" {
		t.Errorf("got %q, want C0123ABCD", got)
	}
}

func TestResolveChannelByNameWithPaging(t *testing.T) {
	mockSlack(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer xoxp-test" {
			t.Errorf("missing/invalid auth header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		cursor := r.URL.Query().Get("cursor")
		if cursor == "" {
			// First page: does not contain the target, returns a cursor.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"channels": []map[string]string{
					{"id": "C0000001", "name": "random"},
					{"id": "C0000002", "name": "general"},
				},
				"response_metadata": map[string]string{"next_cursor": "page2"},
			})
			return
		}
		// Second page: contains the target channel.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"channels": []map[string]string{
				{"id": "C0000003", "name": "standups"},
			},
			"response_metadata": map[string]string{"next_cursor": ""},
		})
	})

	got, err := NewClient("xoxp-test").ResolveChannel(context.Background(), "#standups")
	if err != nil {
		t.Fatalf("ResolveChannel: %v", err)
	}
	if got != "C0000003" {
		t.Errorf("got %q, want C0000003", got)
	}
}

func TestResolveChannelNotFound(t *testing.T) {
	mockSlack(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":                true,
			"channels":          []map[string]string{{"id": "C1", "name": "general"}},
			"response_metadata": map[string]string{"next_cursor": ""},
		})
	})
	if _, err := NewClient("xoxp-test").ResolveChannel(context.Background(), "nope"); err == nil {
		t.Fatal("expected error for missing channel, got nil")
	}
}

func TestResolveChannelAPIError(t *testing.T) {
	mockSlack(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "missing_scope"})
	})
	if _, err := NewClient("xoxp-test").ResolveChannel(context.Background(), "general"); err == nil {
		t.Fatal("expected error from API ok:false, got nil")
	}
}

func TestAuthTest(t *testing.T) {
	mockSlack(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth.test" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"team_id": "T0ABCDEF",
			"team":    "Acme",
			"user":    "tukesh",
			"url":     "https://acme.slack.com/",
		})
	})

	info, err := NewClient("xoxp-test").AuthTest(context.Background())
	if err != nil {
		t.Fatalf("AuthTest: %v", err)
	}
	if info.TeamID != "T0ABCDEF" || info.Team != "Acme" || info.User != "tukesh" {
		t.Errorf("unexpected auth info: %+v", info)
	}
}

// TestEndToEndHandoff exercises the full resolution path the CLI uses: resolve a
// #name to an ID, look up the team ID, and build the links the user is sent to —
// all against the mock Slack server, with OpenURL stubbed.
func TestEndToEndHandoff(t *testing.T) {
	mockSlack(t, func(w http.ResponseWriter, r *http.Request) {
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
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	})

	var opened string
	orig := openURL
	openURL = func(_ context.Context, target string) error { opened = target; return nil }
	t.Cleanup(func() { openURL = orig })

	ctx := context.Background()
	c := NewClient("xoxp-test")

	id, err := c.ResolveChannel(ctx, "#announcements")
	if err != nil {
		t.Fatalf("ResolveChannel: %v", err)
	}
	info, err := c.AuthTest(ctx)
	if err != nil {
		t.Fatalf("AuthTest: %v", err)
	}
	deep, web := ChannelLinks(id, info.TeamID)
	if err := OpenURL(ctx, deep); err != nil {
		t.Fatalf("OpenURL: %v", err)
	}

	wantDeep := "slack://channel?team=T0WORKSPACE&id=C0ANNOUNCE"
	wantWeb := "https://slack.com/app_redirect?channel=C0ANNOUNCE"
	if deep != wantDeep {
		t.Errorf("deep link = %q, want %q", deep, wantDeep)
	}
	if web != wantWeb {
		t.Errorf("web link = %q, want %q", web, wantWeb)
	}
	if opened != wantDeep {
		t.Errorf("OpenURL got %q, want %q", opened, wantDeep)
	}
}

// Package slack implements a "manual approval" hand-off to Slack.
//
// git-brief never posts to Slack on the user's behalf. Instead it resolves the
// target channel, copies the brief to the clipboard, and opens the Slack client
// directly in that channel. The user pastes the brief and presses send
// themselves — so the message is always posted *as the user* (never as a bot)
// and only ever after explicit manual confirmation inside Slack.
package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const defaultAPIBase = "https://slack.com/api"

// channelIDPattern matches Slack conversation IDs such as C0123ABCD, G0123ABCD
// (private channels) and D0123ABCD (DMs). Channel *names* are lowercase and are
// never matched here.
var channelIDPattern = regexp.MustCompile(`^[CGD][A-Z0-9]{6,}$`)

// apiBase returns the Slack Web API base URL. It is overridable via the
// SLACK_API_BASE environment variable so the integration can be tested against
// a local mock server.
func apiBase() string {
	if b := strings.TrimSpace(os.Getenv("SLACK_API_BASE")); b != "" {
		return strings.TrimRight(b, "/")
	}
	return defaultAPIBase
}

// Client is a minimal Slack Web API client. It uses a user token (xoxp-…) only
// to look things up (resolve a channel name to an ID, discover the team ID); it
// never posts messages.
type Client struct {
	token string
	http  *http.Client
}

// NewClient returns a Slack client authenticated with the given token.
func NewClient(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 15 * time.Second},
	}
}

// AuthInfo is the subset of the auth.test response that we use.
type AuthInfo struct {
	TeamID string
	Team   string
	User   string
	URL    string
}

// IsChannelID reports whether s already looks like a Slack conversation ID
// (e.g. "C0123ABCD") rather than a human-readable name like "#standups".
func IsChannelID(s string) bool {
	return channelIDPattern.MatchString(strings.TrimSpace(s))
}

// normalizeName strips a leading '#' and surrounding whitespace from a channel
// name so "#standups" and "standups" compare equal.
func normalizeName(s string) string {
	return strings.TrimPrefix(strings.TrimSpace(s), "#")
}

// ParseChannelURL extracts a channel ID (and team ID, when present) from a Slack
// channel link or deep link that a user copied straight from the Slack app — for
// example "https://acme.slack.com/archives/C0123ABCD" (channel ▸ Copy link),
// "https://app.slack.com/client/T0AAAA/C0123ABCD", or
// "slack://channel?team=T0AAAA&id=C0123ABCD".
//
// ok reports whether s looked like such a URL. This requires no token and no
// admin rights — any workspace member can copy a channel link.
func ParseChannelURL(s string) (channelID, teamID string, ok bool) {
	s = strings.TrimSpace(s)
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" {
		return "", "", false
	}

	switch u.Scheme {
	case "slack":
		q := u.Query()
		return q.Get("id"), q.Get("team"), true
	case "http", "https":
		if !strings.Contains(u.Host, "slack.com") {
			return "", "", false
		}
		if c := u.Query().Get("channel"); c != "" {
			channelID = c // app_redirect?channel=C0123ABCD
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i, p := range parts {
			switch p {
			case "archives":
				if i+1 < len(parts) && IsChannelID(parts[i+1]) {
					channelID = parts[i+1]
				}
			case "client":
				if i+1 < len(parts) && strings.HasPrefix(parts[i+1], "T") {
					teamID = parts[i+1]
				}
				if i+2 < len(parts) && IsChannelID(parts[i+2]) {
					channelID = parts[i+2]
				}
			}
		}
		return channelID, teamID, true
	}
	return "", "", false
}

// ChannelLinks returns a native deep link (slack://) and a web fallback link
// (https://slack.com/app_redirect) that both open the given channel in the
// user's Slack client. The native link is only well-formed when teamID is
// known; otherwise both returned values are the web link.
func ChannelLinks(channelID, teamID string) (deepLink, webLink string) {
	webLink = "https://slack.com/app_redirect?channel=" + url.QueryEscape(channelID)
	if teamID == "" {
		return webLink, webLink
	}
	deepLink = fmt.Sprintf("slack://channel?team=%s&id=%s",
		url.QueryEscape(teamID), url.QueryEscape(channelID))
	return deepLink, webLink
}

// AuthTest calls auth.test and returns identity/team information for the token.
func (c *Client) AuthTest(ctx context.Context) (*AuthInfo, error) {
	var resp struct {
		OK     bool   `json:"ok"`
		Error  string `json:"error"`
		TeamID string `json:"team_id"`
		Team   string `json:"team"`
		User   string `json:"user"`
		URL    string `json:"url"`
	}
	if err := c.get(ctx, "/auth.test", nil, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("slack auth.test: %s", errOrUnknown(resp.Error))
	}
	return &AuthInfo{TeamID: resp.TeamID, Team: resp.Team, User: resp.User, URL: resp.URL}, nil
}

// ResolveChannel returns the conversation ID for the given channel. If channel
// is already an ID it is returned unchanged; otherwise conversations.list is
// paged through to find a public or private channel whose name matches.
func (c *Client) ResolveChannel(ctx context.Context, channel string) (string, error) {
	if IsChannelID(channel) {
		return strings.TrimSpace(channel), nil
	}
	want := normalizeName(channel)
	if want == "" {
		return "", fmt.Errorf("empty channel name")
	}

	cursor := ""
	for {
		var resp struct {
			OK       bool   `json:"ok"`
			Error    string `json:"error"`
			Channels []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"channels"`
			Meta struct {
				NextCursor string `json:"next_cursor"`
			} `json:"response_metadata"`
		}
		q := url.Values{}
		q.Set("types", "public_channel,private_channel")
		q.Set("exclude_archived", "true")
		q.Set("limit", "1000")
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		if err := c.get(ctx, "/conversations.list", q, &resp); err != nil {
			return "", err
		}
		if !resp.OK {
			return "", fmt.Errorf("slack conversations.list: %s", errOrUnknown(resp.Error))
		}
		for _, ch := range resp.Channels {
			if strings.EqualFold(ch.Name, want) {
				return ch.ID, nil
			}
		}
		cursor = resp.Meta.NextCursor
		if cursor == "" {
			break
		}
	}
	return "", fmt.Errorf("channel %q not found (is the token a member of it?)", channel)
}

// get performs an authenticated GET against the Slack Web API and decodes the
// JSON response into out.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := apiBase() + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("slack request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("slack request: unexpected status %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// openURL points at the function used to launch the Slack client. It is a
// package variable so tests can stub it.
var openURL = realOpenURL

// OpenURL opens target (a slack:// deep link or https URL) in the user's
// default handler / browser.
func OpenURL(ctx context.Context, target string) error {
	return openURL(ctx, target)
}

func realOpenURL(ctx context.Context, target string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{target}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", target}
	default:
		name, args = "xdg-open", []string{target}
	}
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Start()
}

func errOrUnknown(s string) string {
	if s == "" {
		return "unknown error"
	}
	return s
}
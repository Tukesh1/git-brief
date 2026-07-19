package output

import "strings"

// ForSlack lightly formats a plain standup brief for Slack mrkdwn so section
// headers stand out when pasted or posted via the API. Bullet lines are left
// as-is (• works in Slack).
func ForSlack(brief string) string {
	lines := strings.Split(strings.TrimSpace(brief), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Yesterday:"):
			out = append(out, "*Yesterday:*"+strings.TrimPrefix(trimmed, "Yesterday:"))
		case strings.HasPrefix(trimmed, "Today:"):
			out = append(out, "*Today:*"+strings.TrimPrefix(trimmed, "Today:"))
		case strings.HasPrefix(trimmed, "Blockers:"):
			out = append(out, "*Blockers:*"+strings.TrimPrefix(trimmed, "Blockers:"))
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

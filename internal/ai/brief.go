package ai

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tukesh1/git-brief/internal/collector"
)

var (
	conventionalCommit = regexp.MustCompile(`(?i)^(feat|fix|docs|refactor|chore|test|ci|perf|build|style)(\([^)]+\))?:\s*(.+)$`)
	sectionHeader      = regexp.MustCompile(`(?m)^(Yesterday|Today|Blockers)\s*:`)
)

// ensureBriefMatchesData keeps the product honest: if the model drops real
// shipped work or skips the required standup shape, replace with a brief
// built directly from commits / PRs / local WIP.
func ensureBriefMatchesData(
	brief string,
	earlier, today []collector.CommitData,
	prs []collector.PRData,
	uncommitted, stashed []string,
) string {
	brief = normalizeBrief(brief)
	if briefFaithful(brief, earlier, today, uncommitted, stashed) {
		return brief
	}
	return buildDeterministicBrief(earlier, today, prs, uncommitted, stashed)
}

func briefFaithful(
	brief string,
	earlier, today []collector.CommitData,
	uncommitted, stashed []string,
) bool {
	if !sectionHeader.MatchString(brief) {
		return false
	}
	if !strings.Contains(brief, "Yesterday:") || !strings.Contains(brief, "Today:") || !strings.Contains(brief, "Blockers:") {
		return false
	}

	yesterdayBody := sectionBody(brief, "Yesterday")
	todayBody := sectionBody(brief, "Today")

	// Shipped work must appear under Yesterday when we have earlier commits.
	if len(earlier) > 0 {
		if bulletCount(yesterdayBody) == 0 {
			return false
		}
		if isPlaceholderYesterday(yesterdayBody) {
			return false
		}
	}

	// Today must mention WIP or today's commits when that is the only signal.
	if len(today) == 0 && (len(uncommitted) > 0 || len(stashed) > 0) {
		if bulletCount(todayBody) == 0 {
			return false
		}
	}
	if len(today) > 0 && bulletCount(todayBody) == 0 && bulletCount(yesterdayBody) == 0 {
		return false
	}

	// Reject vague "updates related to … configuration" when we have rich commit history.
	if len(earlier)+len(today) >= 3 && isVagueBrief(brief) {
		return false
	}
	return true
}

func isPlaceholderYesterday(body string) bool {
	l := strings.ToLower(body)
	return strings.Contains(l, "no shipped") ||
		strings.Contains(l, "no activity") ||
		strings.Contains(l, "no commits")
}

func isVagueBrief(brief string) bool {
	l := strings.ToLower(brief)
	vague := []string{
		"updates related to",
		"working on updates across",
		"various updates",
		"general updates",
		"miscellaneous",
		"continuing work on documentation",
	}
	for _, v := range vague {
		if strings.Contains(l, v) {
			return true
		}
	}
	return false
}

func sectionBody(brief, name string) string {
	lines := strings.Split(brief, "\n")
	var b strings.Builder
	in := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, name+":") {
			in = true
			continue
		}
		if in {
			if strings.HasPrefix(trim, "Yesterday:") || strings.HasPrefix(trim, "Today:") || strings.HasPrefix(trim, "Blockers:") {
				break
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func bulletCount(body string) int {
	n := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "•") {
			n++
		}
	}
	return n
}

// buildDeterministicBrief produces a correct Slack standup from the collected
// data without inventing work. Used when the model output fails the goal.
func buildDeterministicBrief(
	earlier, today []collector.CommitData,
	prs []collector.PRData,
	uncommitted, stashed []string,
) string {
	var b strings.Builder

	b.WriteString("Yesterday:\n")
	yBullets := themeBullets(earlier, 4)
	yBullets = append(yBullets, prBullets(prs, "merged", "reviewed")...)
	if len(yBullets) > 4 {
		yBullets = yBullets[:4]
	}
	if len(yBullets) == 0 {
		b.WriteString("  • No shipped commits since last standup\n")
	} else {
		for _, line := range yBullets {
			fmt.Fprintf(&b, "  • %s\n", line)
		}
	}

	b.WriteString("\nToday:\n")
	tBullets := themeBullets(today, 3)
	tBullets = append(tBullets, prBullets(prs, "draft", "issue")...)
	tBullets = append(tBullets, localWIPBullets(uncommitted, stashed)...)
	if len(tBullets) > 4 {
		tBullets = tBullets[:4]
	}
	if len(tBullets) == 0 {
		b.WriteString("  • No new local WIP\n")
	} else {
		for _, line := range tBullets {
			fmt.Fprintf(&b, "  • %s\n", line)
		}
	}

	b.WriteString("\nBlockers:\n  None\n")
	return strings.TrimSpace(b.String())
}

// themeBullets groups conventional commits by scope/type into outcome lines.
func themeBullets(commits []collector.CommitData, max int) []string {
	if len(commits) == 0 || max <= 0 {
		return nil
	}
	type group struct {
		label string
		msgs  []string
	}
	order := make([]string, 0)
	groups := map[string]*group{}

	for _, c := range commits {
		key, label, subject := commitTheme(c.Message)
		g, ok := groups[key]
		if !ok {
			g = &group{label: label}
			groups[key] = g
			order = append(order, key)
		}
		// Keep distinct subjects only.
		dup := false
		for _, m := range g.msgs {
			if strings.EqualFold(m, subject) {
				dup = true
				break
			}
		}
		if !dup {
			g.msgs = append(g.msgs, subject)
		}
	}

	var out []string
	for _, key := range order {
		if len(out) >= max {
			break
		}
		g := groups[key]
		line := formatThemeBullet(g.label, g.msgs)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func commitTheme(msg string) (key, label, subject string) {
	msg = strings.TrimSpace(msg)
	m := conventionalCommit.FindStringSubmatch(msg)
	if m != nil {
		typ := strings.ToLower(m[1])
		scope := strings.Trim(m[2], "():")
		subject = strings.TrimSpace(m[3])
		if scope != "" {
			return typ + ":" + scope, scope, subject
		}
		return typ, typ, subject
	}
	// Non-conventional: use first 4 words as key, full message as subject.
	words := strings.Fields(msg)
	key = strings.ToLower(msg)
	if len(words) > 4 {
		key = strings.ToLower(strings.Join(words[:4], " "))
	}
	return key, "", msg
}

func formatThemeBullet(label string, msgs []string) string {
	if len(msgs) == 0 {
		return ""
	}
	verbSubject := tidySubject(msgs[0])
	if len(msgs) == 1 {
		if label != "" && !strings.Contains(strings.ToLower(verbSubject), strings.ToLower(label)) {
			return fmt.Sprintf("%s (%s)", ensureVerb(verbSubject), label)
		}
		return ensureVerb(verbSubject)
	}
	// Multiple related commits → one theme line.
	if label != "" {
		return fmt.Sprintf("Shipped %s work: %s", label, joinShort(msgs, 2))
	}
	return ensureVerb(fmt.Sprintf("%s (+%d related)", tidySubject(msgs[0]), len(msgs)-1))
}

func joinShort(msgs []string, n int) string {
	parts := make([]string, 0, n)
	for i, m := range msgs {
		if i >= n {
			break
		}
		parts = append(parts, tidySubject(m))
	}
	s := strings.Join(parts, "; ")
	if len(msgs) > n {
		s += fmt.Sprintf(" (+%d more)", len(msgs)-n)
	}
	return s
}

func tidySubject(s string) string {
	s = strings.TrimSpace(s)
	// Strip trailing PR markers duplication later if needed.
	return strings.TrimSuffix(s, ".")
}

func ensureVerb(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// If it already starts with a common verb / past tense, keep it.
	first := strings.ToLower(strings.Fields(s)[0])
	verbs := map[string]bool{
		"shipped": true, "fixed": true, "added": true, "merged": true, "reviewed": true,
		"finishing": true, "working": true, "updated": true, "implemented": true,
		"refactored": true, "improved": true, "removed": true, "built": true,
		"pushed": true, "wired": true, "made": true, "enabled": true, "tuned": true,
	}
	if verbs[first] {
		return capitalize(s)
	}
	// Conventional leftover "add X" → "Added X"
	if strings.HasPrefix(first, "add") {
		return "Added " + trimFirstWord(s)
	}
	if strings.HasPrefix(first, "fix") {
		return "Fixed " + trimFirstWord(s)
	}
	return capitalize(s)
}

func trimFirstWord(s string) string {
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return s
	}
	return strings.Join(parts[1:], " ")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func prBullets(prs []collector.PRData, types ...string) []string {
	allow := map[string]bool{}
	for _, t := range types {
		allow[t] = true
	}
	var out []string
	for _, p := range prs {
		if !allow[p.Type] {
			continue
		}
		switch p.Type {
		case "merged":
			out = append(out, fmt.Sprintf("Merged PR #%d in %s: %s", p.Number, p.RepoName, p.Title))
		case "reviewed":
			out = append(out, fmt.Sprintf("Reviewed PR #%d in %s: %s", p.Number, p.RepoName, p.Title))
		case "draft":
			out = append(out, fmt.Sprintf("Working on draft PR #%d in %s: %s", p.Number, p.RepoName, p.Title))
		case "issue":
			out = append(out, fmt.Sprintf("Following issue #%d in %s: %s", p.Number, p.RepoName, p.Title))
		}
	}
	return out
}

func localWIPBullets(uncommitted, stashed []string) []string {
	var out []string
	for _, line := range uncommitted {
		out = append(out, "Finishing local WIP — "+compactUncommittedLine(line))
	}
	for _, line := range stashed {
		out = append(out, "Resuming stashed work — "+line)
	}
	return out
}

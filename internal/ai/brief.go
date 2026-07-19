package ai

import (
	"fmt"
	"path/filepath"
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

	if len(earlier) > 0 {
		if bulletCount(yesterdayBody) == 0 || isPlaceholderYesterday(yesterdayBody) {
			return false
		}
	}

	meaningfulWIP := hasMeaningfulWIP(uncommitted)
	if len(today) == 0 && meaningfulWIP && bulletCount(todayBody) == 0 {
		return false
	}
	if len(today) == 0 && !meaningfulWIP && len(stashed) == 0 && len(earlier) > 0 {
		// Yesterday-only day is fine.
	}

	if len(earlier)+len(today) >= 3 && isVagueBrief(brief) {
		return false
	}
	// Tool jargon means the model (or old formatter) leaked — rebuild.
	if isToolJargonBrief(brief) {
		return false
	}
	return true
}

func hasMeaningfulWIP(uncommitted []string) bool {
	for _, line := range uncommitted {
		if humanWIPBullet(line) != "" {
			return true
		}
	}
	return false
}

func isPlaceholderYesterday(body string) bool {
	l := strings.ToLower(body)
	return strings.Contains(l, "no shipped") ||
		strings.Contains(l, "no activity") ||
		strings.Contains(l, "no commits")
}

func isVagueBrief(brief string) bool {
	l := strings.ToLower(brief)
	for _, v := range []string{
		"updates related to",
		"working on updates across",
		"various updates",
		"general updates",
		"miscellaneous",
		"continuing work on documentation",
	} {
		if strings.Contains(l, v) {
			return true
		}
	}
	return false
}

func isToolJargonBrief(brief string) bool {
	l := strings.ToLower(brief)
	for _, v := range []string{
		"dirty files",
		"sample:",
		"local wip —",
		"~1 dirty",
		"...and ",
	} {
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

// buildDeterministicBrief produces a Slack standup in spoken eng language.
func buildDeterministicBrief(
	earlier, today []collector.CommitData,
	prs []collector.PRData,
	uncommitted, stashed []string,
) string {
	var b strings.Builder

	b.WriteString("Yesterday:\n")
	yBullets := themeBullets(earlier, 4, false)
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
	tBullets := themeBullets(today, 3, true)
	tBullets = append(tBullets, prBullets(prs, "draft", "issue")...)
	tBullets = append(tBullets, localWIPBullets(uncommitted, stashed)...)
	if len(tBullets) > 4 {
		tBullets = tBullets[:4]
	}
	if len(tBullets) == 0 {
		b.WriteString("  • No open local work\n")
	} else {
		for _, line := range tBullets {
			fmt.Fprintf(&b, "  • %s\n", line)
		}
	}

	b.WriteString("\nBlockers:\n  None\n")
	return strings.TrimSpace(b.String())
}

func themeBullets(commits []collector.CommitData, max int, forToday bool) []string {
	if len(commits) == 0 || max <= 0 {
		return nil
	}
	type group struct {
		topic string
		repo  string
		msgs  []string
	}
	order := make([]string, 0)
	groups := map[string]*group{}

	for _, c := range commits {
		_, _, subject := commitTheme(c.Message)
		topic := topicKey(subject)
		key := c.Repo + "|" + topic
		g, ok := groups[key]
		if !ok {
			g = &group{topic: topic, repo: c.Repo}
			groups[key] = g
			order = append(order, key)
		}
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
		line := formatTopicBullet(g.topic, g.repo, g.msgs, forToday)
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
	return strings.ToLower(msg), "", msg
}

// topicKey clusters related commit subjects so landing-page work becomes one
// complete bullet instead of three thin fragments.
func topicKey(subject string) string {
	l := strings.ToLower(subject)
	topics := []struct {
		name string
		keys []string
	}{
		{"landing page", []string{"landing"}},
		{"railway deploy", []string{"railway"}},
		{"null safety", []string{"null check", "null-safety", "nil check"}},
		{"slack standup", []string{"slack"}},
		{"contributors", []string{"contributor", "contribution pulse"}},
		{"docs rendering", []string{"markdown", "docs as markdown", "render docs"}},
		{"overview", []string{"overview", "github panel"}},
		{"explore", []string{"explore intelligence", "quiz", "graph, concepts"}},
		{"auth", []string{"auth", "login", "oauth"}},
		{"ci", []string{"workflow", "ci ", "github actions"}},
		{"standup cli", []string{"git-brief", "standup", "clipboard"}},
	}
	for _, t := range topics {
		for _, k := range t.keys {
			if strings.Contains(l, k) {
				return t.name
			}
		}
	}
	// Fallback: conventional scope or first 3 words
	if m := conventionalCommit.FindStringSubmatch(subject); m != nil {
		if scope := strings.Trim(m[2], "():"); scope != "" {
			return strings.ToLower(scope)
		}
	}
	words := strings.Fields(l)
	if len(words) > 3 {
		words = words[:3]
	}
	return strings.Join(words, " ")
}

func formatTopicBullet(topic, repo string, msgs []string, forToday bool) string {
	if len(msgs) == 0 {
		return ""
	}
	body := completeTopicPhrase(topic, msgs, forToday)
	if body == "" {
		return ""
	}
	if repo != "" && !strings.Contains(strings.ToLower(body), strings.ToLower(repo)) {
		body += " in " + repo
	}
	return body
}

// completeTopicPhrase builds a teammate-readable bullet from a topic + subjects.
func completeTopicPhrase(topic string, msgs []string, forToday bool) string {
	details := uniqueDetails(msgs)

	switch topic {
	case "landing page":
		if forToday {
			return "Finishing landing page updates" + detailSuffix(details, []string{"font", "new page", "layout", "cta"})
		}
		return "Updated the landing page" + detailSuffix(details, []string{"new page", "font size", "layout", "cta", "polish"})
	case "railway deploy":
		if forToday {
			return "Working on Railway deployment configuration"
		}
		return "Added Railway deployment configuration"
	case "null safety":
		if forToday {
			return "Finishing a null-safety fix"
		}
		return "Fixed a null-safety issue"
	case "slack standup":
		if forToday {
			return "Finishing Slack standup delivery"
		}
		return "Shipped Slack standup delivery" + detailSuffix(details, []string{"post", "token", "clipboard", "hand-off", "channel"})
	case "contributors":
		if forToday {
			return "Fixing Contributors / Contribution pulse rendering"
		}
		return "Fixed Contributors and Contribution pulse rendering"
	case "docs rendering":
		if forToday {
			return "Polishing docs markdown rendering and settings flows"
		}
		return "Improved docs markdown rendering and settings / add-project flows"
	case "overview":
		if forToday {
			return "Working on project overview and GitHub panels"
		}
		return "Improved project overview and GitHub panel reliability"
	case "explore":
		if forToday {
			return "Building Explore intelligence features"
		}
		return "Added Explore intelligence (graph, concepts, quiz, notes)"
	case "auth":
		if forToday {
			return "Working on auth / login"
		}
		return "Improved auth and login flows"
	case "ci":
		if forToday {
			return "Updating CI workflows"
		}
		return "Updated CI workflows"
	case "standup cli":
		if forToday {
			return "Finishing git-brief standup CLI improvements"
		}
		return "Improved git-brief standup CLI"
	}

	// Generic: one richer sentence from the best subject + optional extras.
	primary := enrichFragment(msgs[0], forToday)
	if len(msgs) == 1 {
		return primary
	}
	extra := summarizeExtras(msgs[1:])
	if extra == "" {
		return primary
	}
	return primary + " — also " + extra
}

func uniqueDetails(msgs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range msgs {
		m = strings.TrimSpace(strings.ToLower(m))
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

func detailSuffix(details []string, hints []string) string {
	var hit []string
	for _, d := range details {
		for _, h := range hints {
			if strings.Contains(d, h) {
				switch {
				case strings.Contains(d, "font"):
					hit = append(hit, "larger fonts")
				case strings.Contains(d, "new page"):
					hit = append(hit, "new page")
				case strings.Contains(d, "null"):
					// skip in landing
				case strings.Contains(h, "post"):
					hit = append(hit, "API post + channel hand-off")
				case strings.Contains(h, "token"):
					hit = append(hit, "optional token")
				default:
					// use short hint label once
				}
				break
			}
		}
	}
	hit = uniqStrings(hit)
	if len(hit) == 0 {
		if len(details) > 1 {
			return " (" + strconvMin(len(details)) + " related changes)"
		}
		return ""
	}
	if len(hit) == 1 {
		return " (" + hit[0] + ")"
	}
	return " (" + strings.Join(hit[:2], ", ") + ")"
}

func strconvMin(n int) string {
	return fmt.Sprintf("%d", n)
}

func uniqStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func summarizeExtras(msgs []string) string {
	if len(msgs) == 0 {
		return ""
	}
	// One short clause from the next subject.
	s := stripLeadingVerb(tidySubject(msgs[0]))
	if s == "" {
		return ""
	}
	if len(msgs) > 1 {
		return lowercaseFirst(s) + fmt.Sprintf(" (+%d more)", len(msgs)-1)
	}
	return lowercaseFirst(s)
}

func stripLeadingVerb(s string) string {
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return s
	}
	switch strings.ToLower(fields[0]) {
	case "added", "add", "fixed", "fix", "updated", "update", "shipped", "improved",
		"implemented", "enabled", "removed", "refactored", "made", "built", "increased", "increase":
		return strings.Join(fields[1:], " ")
	}
	return s
}

// enrichFragment turns thin commit subjects into complete standup phrases.
func enrichFragment(subject string, forToday bool) string {
	s := tidySubject(stripCommitNoise(subject))
	if s == "" {
		return ""
	}
	l := strings.ToLower(s)

	// Specific expansions for common thin subjects
	replacements := []struct {
		match string
		past  string
		today string
	}{
		{"railway configuration", "Added Railway deployment configuration", "Working on Railway deployment configuration"},
		{"null check", "Fixed a null-safety issue", "Finishing a null-safety fix"},
		{"font size of landing page", "Increased landing page font sizes", "Adjusting landing page font sizes"},
		{"new landing page", "Shipped the new landing page", "Finishing the new landing page"},
		{"updated and added the new landing page", "Updated and shipped the new landing page", "Finishing the new landing page"},
	}
	for _, r := range replacements {
		if strings.Contains(l, r.match) {
			if forToday {
				return r.today
			}
			return r.past
		}
	}

	fields := strings.Fields(s)
	first := strings.ToLower(fields[0])
	rest := strings.Join(fields[1:], " ")

	if forToday {
		switch first {
		case "added", "add", "updated", "update", "fixed", "fix", "shipped", "improved", "increased", "increase":
			if rest == "" {
				return "Finishing " + lowercaseFirst(s)
			}
			return "Finishing " + rest
		case "finishing", "working":
			return capitalize(s)
		default:
			return "Working on " + lowercaseFirst(s)
		}
	}

	switch first {
	case "add":
		return "Added " + rest
	case "fix":
		return "Fixed " + rest
	case "update":
		return "Updated " + rest
	case "increase":
		return "Increased " + rest
	case "deliver", "ship":
		return "Shipped " + rest
	case "added", "fixed", "updated", "increased", "shipped", "improved", "implemented",
		"enabled", "removed", "refactored", "made", "built", "rendered", "contained", "complete", "build":
		return capitalize(s)
	default:
		// Bare fragment like "railway configuration"
		if first != "" && !strings.HasSuffix(first, "ed") && len(fields) <= 4 {
			return "Completed " + lowercaseFirst(s)
		}
		return capitalize(s)
	}
}

func stripCommitNoise(s string) string {
	s = strings.TrimSpace(s)
	// Drop trailing PR numbers already unusual in subjects.
	s = regexp.MustCompile(`\s*\(#\d+\)\s*$`).ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func tidySubject(s string) string {
	return strings.TrimSuffix(strings.TrimSpace(s), ".")
}

func lowercaseFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
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
			out = append(out, fmt.Sprintf("Merged PR #%d (%s)", p.Number, p.Title))
		case "reviewed":
			out = append(out, fmt.Sprintf("Reviewed PR #%d (%s)", p.Number, p.Title))
		case "draft":
			out = append(out, fmt.Sprintf("Working on draft PR #%d (%s)", p.Number, p.Title))
		case "issue":
			out = append(out, fmt.Sprintf("Following up on #%d (%s)", p.Number, p.Title))
		}
	}
	return out
}

func localWIPBullets(uncommitted, stashed []string) []string {
	var out []string
	for _, line := range uncommitted {
		if bullet := humanWIPBullet(line); bullet != "" {
			out = append(out, bullet)
		}
	}
	for _, line := range stashed {
		// stash messages are often already human ("WIP on feature/x")
		msg := line
		if _, rest, ok := strings.Cut(line, ": "); ok {
			msg = rest
		}
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}
		out = append(out, "Picking up stashed work: "+tidySubject(msg))
	}
	return out
}

// humanWIPBullet turns "repo: a.go, b.go" into a spoken Today line.
// Returns "" when only noise paths remain.
func humanWIPBullet(line string) string {
	repo, filesPart, ok := strings.Cut(line, ": ")
	if !ok {
		return ""
	}
	repo = strings.TrimSpace(repo)
	files := collector.MeaningfulFiles(splitFileList(filesPart))
	if len(files) == 0 {
		return ""
	}
	area := inferWorkArea(files)
	if area != "" {
		return fmt.Sprintf("Working on %s in %s", area, repo)
	}
	return fmt.Sprintf("Working on %s in %s", shortPathPhrase(files), repo)
}

func inferWorkArea(files []string) string {
	joined := strings.ToLower(strings.Join(files, " "))
	switch {
	case strings.Contains(joined, "slack"):
		return "Slack standup delivery"
	case strings.Contains(joined, "landing"):
		return "the landing page"
	case strings.Contains(joined, "auth") || strings.Contains(joined, "login"):
		return "auth"
	case strings.Contains(joined, "internal/ai") || strings.Contains(joined, "summarize") || strings.Contains(joined, "system_prompt"):
		return "standup generation"
	case strings.Contains(joined, "cmd/") || strings.Contains(joined, "makefile"):
		return "the CLI"
	case strings.Contains(joined, "workflow") || strings.Contains(joined, ".github"):
		return "CI"
	case strings.Contains(joined, "readme") || strings.Contains(joined, "docs/"):
		return "docs"
	case strings.Contains(joined, "config"):
		return "config"
	}

	// Dominant top-level / second-level directory
	counts := map[string]int{}
	for _, f := range files {
		f = filepath.ToSlash(f)
		parts := strings.Split(f, "/")
		key := parts[0]
		if len(parts) > 1 && (key == "internal" || key == "src" || key == "app" || key == "pkg") {
			key = parts[0] + "/" + parts[1]
		}
		counts[key]++
	}
	best, bestN := "", 0
	for k, n := range counts {
		if n > bestN {
			best, bestN = k, n
		}
	}
	if best != "" && bestN >= 2 {
		return best
	}
	return ""
}

func shortPathPhrase(files []string) string {
	if len(files) == 1 {
		return filepath.ToSlash(files[0])
	}
	if len(files) == 2 {
		return filepath.ToSlash(files[0]) + " and " + filepath.ToSlash(files[1])
	}
	return fmt.Sprintf("%s and %d other files", filepath.ToSlash(files[0]), len(files)-1)
}

// humanWIPLinesForPrompt formats local WIP for the LLM in spoken language
// (no "dirty files" jargon).
func humanWIPLinesForPrompt(uncommitted, stashed []string) []string {
	return localWIPBullets(uncommitted, stashed)
}

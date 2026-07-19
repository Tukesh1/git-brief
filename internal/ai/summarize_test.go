package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/tukesh1/git-brief/internal/collector"
)

func TestBucketCommits(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.Local)
	commits := []collector.CommitData{
		{Repo: "r", Branch: "b", Message: "old work", Date: "2026-06-30T04:22:10+05:30"},
		{Repo: "r", Branch: "b", Message: "today work", Date: "2026-07-19T09:00:00+05:30"},
	}
	earlier, todayItems := bucketCommits(commits, now)
	if len(earlier) != 1 || todayItems[0].Message != "today work" {
		t.Fatalf("bucket failed: earlier=%v today=%v", earlier, todayItems)
	}
}

func TestCompleteStandupBulletsFromThinCommits(t *testing.T) {
	earlier := []collector.CommitData{
		{Repo: "codexp-ai", Message: "added railway configuration"},
		{Repo: "codexp-ai", Message: "added a null check"},
		{Repo: "codexp-ai", Message: "increased font size of landing page"},
		{Repo: "codexp-ai", Message: "updated and added the new landing page"},
	}
	got := buildDeterministicBrief(earlier, nil, nil, nil, nil)
	y := sectionBody(got, "Yesterday")

	if strings.Contains(y, "Added a null check") && !strings.Contains(strings.ToLower(y), "null-safety") {
		t.Fatalf("null check not enriched:\n%s", got)
	}
	if strings.Contains(y, "Increased font size of landing page") {
		t.Fatalf("thin landing fragment should be merged/enriched:\n%s", got)
	}
	if !strings.Contains(strings.ToLower(y), "landing page") {
		t.Fatalf("expected landing page theme:\n%s", got)
	}
	if !strings.Contains(strings.ToLower(y), "railway") {
		t.Fatalf("expected railway theme:\n%s", got)
	}
	if !strings.Contains(y, "codexp-ai") {
		t.Fatalf("expected repo context:\n%s", got)
	}
	// Prefer fewer richer bullets
	if bulletCount(y) > 4 {
		t.Fatalf("too many thin bullets: %d\n%s", bulletCount(y), got)
	}
}

func TestHumanWIPFromRealPaths(t *testing.T) {
	bullet := humanWIPBullet("git-brief: internal/slack/slack.go, cmd/root.go, Makefile")
	if bullet != "Working on Slack standup delivery in git-brief" {
		t.Fatalf("got %q", bullet)
	}
}

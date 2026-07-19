package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/tukesh1/git-brief/internal/collector"
)

func TestCompactUncommittedLine(t *testing.T) {
	in := "git-brief: Makefile, cmd/init.go, README.md, go.mod, AGENTS.md, ...and 12 more"
	got := compactUncommittedLine(in)
	if !strings.Contains(got, "~17 dirty files") {
		t.Errorf("got %q, want ~17 dirty files", got)
	}
}

func TestBucketCommits(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.Local)
	commits := []collector.CommitData{
		{Repo: "r", Branch: "b", Message: "old work", Date: "2026-06-30T04:22:10+05:30"},
		{Repo: "r", Branch: "b", Message: "today work", Date: "2026-07-19T09:00:00+05:30"},
	}
	earlier, todayItems := bucketCommits(commits, now)
	if len(earlier) != 1 || earlier[0].Message != "old work" {
		t.Fatalf("earlier=%v", earlier)
	}
	if len(todayItems) != 1 || todayItems[0].Message != "today work" {
		t.Fatalf("today=%v", todayItems)
	}
}

func TestEnsureBriefFallsBackWhenModelDropsYesterday(t *testing.T) {
	earlier := []collector.CommitData{
		{Message: "feat(slack): add background chat posting", Date: "2026-06-29T15:00:24+05:30"},
		{Message: "feat(slack): post as user", Date: "2026-06-29T15:05:10+05:30"},
		{Message: "feat(cmd): make Slack token optional", Date: "2026-06-29T14:56:25+05:30"},
	}
	bad := "Today:\n  • Working on updates related to configuration.\n\nBlockers:\n  None"
	got := ensureBriefMatchesData(bad, earlier, nil, nil, []string{"git-brief: cmd/init.go"}, nil)
	if !strings.Contains(got, "Yesterday:") {
		t.Fatalf("expected Yesterday section, got:\n%s", got)
	}
	if !strings.Contains(strings.ToLower(got), "slack") {
		t.Fatalf("expected slack theme from commits, got:\n%s", got)
	}
	if strings.Contains(got, "updates related to") {
		t.Fatalf("vague model text should not survive:\n%s", got)
	}
}

func TestDeterministicBriefIncludesCommitsAndWIP(t *testing.T) {
	earlier := []collector.CommitData{
		{Message: "feat(slack): add background chat posting", Date: "2026-06-29T15:00:24+05:30"},
	}
	got := buildDeterministicBrief(earlier, nil, nil, []string{"git-brief: cmd/init.go, README.md"}, nil)
	if !strings.Contains(got, "Yesterday:") || !strings.Contains(got, "Today:") {
		t.Fatal(got)
	}
	if !strings.Contains(strings.ToLower(got), "slack") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "Finishing local WIP") {
		t.Fatal(got)
	}
}

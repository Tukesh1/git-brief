package collector

import "testing"

func TestIsNoisePath(t *testing.T) {
	noise := []string{
		".claude/settings.json",
		".cursor/rules/foo.md",
		"node_modules/lodash/index.js",
		"dist/bundle.js",
		"__pycache__/x.pyc",
		".DS_Store",
		"package-lock.json",
		"go.sum",
		".env.local",
	}
	for _, p := range noise {
		if !IsNoisePath(p) {
			t.Errorf("expected noise: %s", p)
		}
	}
	real := []string{
		"cmd/root.go",
		"internal/slack/slack.go",
		"src/components/Landing.tsx",
		".github/workflows/go.yml",
		"README.md",
		"docs/guides/setup.md",
	}
	for _, p := range real {
		if IsNoisePath(p) {
			t.Errorf("expected meaningful: %s", p)
		}
	}
}

func TestMeaningfulFiles(t *testing.T) {
	got := MeaningfulFiles([]string{
		".claude/foo",
		"cmd/init.go",
		"node_modules/x",
		"internal/ai/brief.go",
		"...and 3 more",
	})
	if len(got) != 2 || got[0] != "cmd/init.go" || got[1] != "internal/ai/brief.go" {
		t.Fatalf("got %#v", got)
	}
}

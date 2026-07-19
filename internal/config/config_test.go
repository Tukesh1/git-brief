package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWeekdayDaysBack(t *testing.T) {
	cases := map[time.Weekday]int{
		time.Monday:    3,
		time.Sunday:    3,
		time.Saturday:  1,
		time.Tuesday:   1,
		time.Wednesday: 1,
		time.Thursday:  1,
		time.Friday:    1,
	}
	for wd, want := range cases {
		if got := weekdayDaysBack(wd); got != want {
			t.Errorf("weekdayDaysBack(%s) = %d, want %d", wd, got, want)
		}
	}
}

func TestSinceTimeWithDays(t *testing.T) {
	got := SinceTime(3)
	wantDay := time.Now().AddDate(0, 0, -3)
	if got.Year() != wantDay.Year() || got.Month() != wantDay.Month() || got.Day() != wantDay.Day() {
		t.Errorf("SinceTime(3) date = %s, want calendar day of %s", got.Format("2006-01-02"), wantDay.Format("2006-01-02"))
	}
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
		t.Errorf("SinceTime(3) should be midnight, got %s", got.Format(time.RFC3339))
	}
}

func TestSinceDescription(t *testing.T) {
	if got := SinceDescription(1); got != "since yesterday" {
		t.Errorf("SinceDescription(1) = %q", got)
	}
	if got := SinceDescription(3); got != "for the last 3 days" {
		t.Errorf("SinceDescription(3) = %q", got)
	}
}

func TestSaveConfigPermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".config", "git-brief"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	Cfg = Config{LLMProvider: "gemini", GeminiAPIKey: "test-key"}
	if err := SaveConfig(); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("config mode = %04o, want 0600 (path %s)", mode, filepath.Base(path))
	}
}

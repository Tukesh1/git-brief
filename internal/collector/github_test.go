package collector

import "testing"

func TestRepoNameFromURL(t *testing.T) {
	cases := map[string]string{
		"https://api.github.com/repos/tukesh1/git-brief":  "tukesh1/git-brief",
		"https://api.github.com/repos/tukesh1/git-brief/": "tukesh1/git-brief",
		"owner/repo": "owner/repo",
	}
	for in, want := range cases {
		if got := repoNameFromURL(in); got != want {
			t.Errorf("repoNameFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

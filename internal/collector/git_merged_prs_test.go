package collector

import "testing"

func TestPRMergeSubject(t *testing.T) {
	cases := []struct {
		subject string
		num     string
		ref     string
		ok      bool
	}{
		{"Merge pull request #6 from Tukesh1/slack-integration", "6", "Tukesh1/slack-integration", true},
		{"Merge pull request #42 from org/feature-x", "42", "org/feature-x", true},
		{"merge pull request #1 from a/b", "1", "a/b", true},
		{"Merge origin/main into slack-integration", "", "", false},
		{"Ship feature", "", "", false},
	}
	for _, tc := range cases {
		m := prMergeSubject.FindStringSubmatch(tc.subject)
		if tc.ok && m == nil {
			t.Fatalf("expected match for %q", tc.subject)
		}
		if !tc.ok {
			if m != nil {
				t.Fatalf("expected no match for %q", tc.subject)
			}
			continue
		}
		if m[1] != tc.num || m[2] != tc.ref {
			t.Fatalf("%q => num=%q ref=%q, want %q / %q", tc.subject, m[1], m[2], tc.num, tc.ref)
		}
	}
}

func TestIsLocalAuthor(t *testing.T) {
	if !isLocalAuthor("Tukesh Kumar", "a@b.com", "Tukesh Kumar", "") {
		t.Fatal("name match failed")
	}
	if !isLocalAuthor("x", "A@B.com", "", "a@b.com") {
		t.Fatal("email match should be case-insensitive")
	}
	if isLocalAuthor("Other", "other@x.com", "Tukesh", "tukesh@x.com") {
		t.Fatal("should not match unrelated author")
	}
}

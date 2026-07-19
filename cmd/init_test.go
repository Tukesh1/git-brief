package cmd

import "testing"

func TestResolveSecretInput(t *testing.T) {
	cases := []struct {
		input, existing, want string
	}{
		{"", "", ""},
		{"", "sk-old", "sk-old"},
		{"   ", "sk-old", "sk-old"},
		{"sk-new", "sk-old", "sk-new"},
		{"sk-new", "", "sk-new"},
	}
	for _, c := range cases {
		if got := resolveSecretInput(c.input, c.existing); got != c.want {
			t.Errorf("resolveSecretInput(%q, %q) = %q, want %q", c.input, c.existing, got, c.want)
		}
	}
}

package cmd

import "testing"

func TestMaskKey(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "(not set)"},
		{"short", "****"},
		{"12345678", "****"},
		{"sk-ant-api-key-value", "sk-ant-a****"},
	}
	for _, c := range cases {
		if got := maskKey(c.in); got != c.want {
			t.Errorf("maskKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

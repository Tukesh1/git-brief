package output

import "testing"

func TestForSlack(t *testing.T) {
	in := "Yesterday:\n  • Shipped Slack delivery\nToday:\n  • Finishing init keep-existing secrets\n\nBlockers:\n  None"
	got := ForSlack(in)
	want := "*Yesterday:*\n  • Shipped Slack delivery\n*Today:*\n  • Finishing init keep-existing secrets\n\n*Blockers:*\n  None"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

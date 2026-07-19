package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

var (
	titleStyle   = color.New(color.FgHiGreen, color.Bold)
	headerStyle  = color.New(color.FgHiCyan, color.Bold)
	bulletStyle  = color.New(color.FgWhite)
	dimStyle     = color.New(color.FgHiBlack)
	successStyle = color.New(color.FgCyan)
	errorStyle   = color.New(color.FgRed)
)

// PrintBrief prints the brief to stdout with section-header colouring.
func PrintBrief(brief string) {
	fmt.Println()
	titleStyle.Printf("📋 brief — %s\n", time.Now().Format("Monday, January 2"))
	dimStyle.Println("─────────────────────────────────")
	fmt.Println()

	lines := strings.Split(strings.TrimSpace(brief), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case isSectionHeader(trimmed):
			headerStyle.Println(line)
		case strings.HasPrefix(trimmed, "•"):
			bulletStyle.Println(line)
		case trimmed == "":
			fmt.Println()
		default:
			fmt.Println(line)
		}
	}

	fmt.Println()
	dimStyle.Println("─────────────────────────────────")
}

// CopyToClipboard copies the brief text and prints a status message.
func CopyToClipboard(brief string) {
	text := strings.TrimSpace(brief)
	if err := copyToClipboard(text); err != nil {
		errorStyle.Printf("⚠️  Could not copy to clipboard: %v\n", err)
	} else {
		successStyle.Println("📋 Copied to clipboard")
	}
}

// isSectionHeader detects standup section headers like "Yesterday:", "Today:", etc.
func isSectionHeader(s string) bool {
	for _, prefix := range []string{"Yesterday:", "Today:", "Blockers:", "Yesterday", "Today", "Blockers"} {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

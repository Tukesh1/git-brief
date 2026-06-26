package output

import "github.com/atotto/clipboard"

// copyToClipboard writes text to the system clipboard.
// Extracted into its own file so it can be stubbed / tested independently.
func copyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

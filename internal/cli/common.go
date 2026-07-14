package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func writeJSON(stdout io.Writer, value any) error {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

// printLeadingCommand writes a blank line then a next-step command hint on stdout.
func printLeadingCommand(w io.Writer, text string) {
	fmt.Fprintf(w, "\n%s\n", text)
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "..."
}

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

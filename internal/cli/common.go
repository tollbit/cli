package cli

import (
	"encoding/json"
	"io"
	"strings"
)

func writeJSON(stdout io.Writer, value any) error {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
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

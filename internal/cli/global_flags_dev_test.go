//go:build dev

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestDevHelpOmitsDisabledGlobalConfigFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"--help"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	for _, flag := range []string{"--auth-base-url", "--gateway-base-url", "--auth-browser-consent-callback-address", "--auth-use-refresh-tokens", "--credentials-storage-dir"} {
		if strings.Contains(stdout.String(), flag) {
			t.Fatalf("expected dev help to omit %s, got %q", flag, stdout.String())
		}
	}
}

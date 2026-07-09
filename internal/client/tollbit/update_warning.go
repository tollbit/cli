package tollbit

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// updateWarningHeader is set by the backend when this CLI version is outdated
// but still supported.
const updateWarningHeader = "X-Tollbit-Cli-Warning"

var (
	updateWarnOnce sync.Once
	// updateWarnWriter is a var so tests can capture the output.
	updateWarnWriter io.Writer = os.Stderr
)

// notifyUpdateWarning prints the server-provided update notice at most once
// per process, on stderr so machine-readable stdout stays clean.
func notifyUpdateWarning(headers http.Header) {
	msg := strings.TrimSpace(headers.Get(updateWarningHeader))
	if msg == "" {
		return
	}
	updateWarnOnce.Do(func() {
		fmt.Fprintln(updateWarnWriter, msg)
	})
}

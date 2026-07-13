package tollbit

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/tollbit/tollbit-cli/internal/installmethod"
)

// latestVersionHeader is set by the backend when this CLI version is outdated
// but still supported. The update message is composed locally because the
// right remediation command depends on how the CLI was installed.
const latestVersionHeader = "X-Tollbit-Cli-Latest-Version"

var (
	updateWarnOnce sync.Once
	// updateWarnWriter is a var so tests can capture the output.
	updateWarnWriter io.Writer = os.Stderr
)

// notifyUpdateWarning prints an install-method-aware update notice at most
// once per process, on stderr so machine-readable stdout stays clean.
func notifyUpdateWarning(headers http.Header) {
	latest := strings.TrimSpace(headers.Get(latestVersionHeader))
	if latest == "" {
		return
	}
	updateWarnOnce.Do(func() {
		fmt.Fprintln(updateWarnWriter, installmethod.UpdateInstructions(installmethod.Detect(), latest))
	})
}

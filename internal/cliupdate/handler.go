// Package cliupdate keeps the CLI up to date: it checks the server's version
// policy, tells the user when a newer version exists, and replaces or stops a
// version below the supported minimum.
package cliupdate

import (
	"fmt"
	"io"
	"sync"

	"github.com/launchdarkly/go-semver"
	"github.com/tollbit/cli/internal/client/versionpolicy"
	"github.com/tollbit/cli/internal/installmethod"
	"github.com/tollbit/cli/internal/version"
)

type (
	// Handler decides what version information means for this run. The
	// cached update check hands it the full policy; gateway responses hand it
	// only an upgrade notice. Either way the notice prints at most once.
	Handler struct {
		out        io.Writer
		noticeOnce sync.Once
		// current is the running version.
		current    string
		autoUpdate bool
		// executable is the binary to update; os.Executable() when empty.
		executable string
		// installers holds the installer for each install method that can
		// update itself, set by ConfigureSelfInstall.
		installers map[installmethod.Method]installer
	}

	// RequiredError stops a command because this version is below the
	// supported minimum. Its message is the install-method-aware update
	// instruction.
	RequiredError struct {
		Minimum string
		Latest  string
	}
)

// NewHandler returns a Handler that writes notices to out, normally stderr.
func NewHandler(out io.Writer) *Handler {
	return &Handler{out: out, current: version.String()}
}

// HandleVersionPolicy returns a RequiredError below the policy minimum and
// prints the upgrade notice below latest. It compares versions the same way
// the gateway does (semver precedence, so 0.4.0-preview.1 sorts below 0.4.0);
// an unparsable version or bound is not enforced.
func (h *Handler) HandleVersionPolicy(policy versionpolicy.Policy) error {
	current, err := semver.Parse(h.current)
	if err != nil {
		return nil
	}
	if versionBelow(current, policy.Minimum) {
		return RequiredError{Minimum: policy.Minimum, Latest: policy.Latest}
	}
	if versionBelow(current, policy.Latest) {
		h.HandleVersionUpgradeNotice(policy.Latest)
	}
	return nil
}

// HandleVersionUpgradeNotice prints the notice that latest is available, at
// most once per Handler.
func (h *Handler) HandleVersionUpgradeNotice(latest string) {
	h.noticeOnce.Do(func() {
		fmt.Fprintln(h.out, installmethod.UpdateInstructions(installmethod.Detect(), latest))
	})
}

func (e RequiredError) Error() string {
	return installmethod.RequiredInstructions(installmethod.Detect(), e.Minimum, e.Latest)
}

func versionBelow(v semver.Version, threshold string) bool {
	if threshold == "" {
		return false
	}
	t, err := semver.Parse(threshold)
	if err != nil {
		return false
	}
	return v.ComparePrecedence(t) < 0
}

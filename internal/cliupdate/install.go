package cliupdate

// This file decides whether the CLI installs a required update itself, and
// asks main to run the command again on it when it does.

import (
	"context"
	"fmt"
	"os"

	"github.com/launchdarkly/go-semver"
	"github.com/rs/zerolog"
	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/installmethod"
	"github.com/tollbit/cli/internal/selfinstall"
	"github.com/tollbit/cli/internal/selfinstall/npm"
	"github.com/tollbit/cli/internal/selfinstall/release"
)

type (
	// RestartError asks main to run the command again on the binary this
	// run just updated. Its Error is only seen if main does not handle it.
	RestartError struct {
		Path    string
		Version string
	}

	// installer updates the CLI binary from current to target the way one
	// install method allows.
	installer interface {
		Install(ctx context.Context, current, target string) (selfinstall.Result, error)
	}
)

func (e RestartError) Error() string {
	return fmt.Sprintf("the TollBit CLI was updated to %s; run the command again", e.Version)
}

// Restart runs the command again on the updated binary and returns its exit
// code. On macOS and Linux it replaces this process and returns only on
// failure.
func (e RestartError) Restart() (int, error) {
	return selfinstall.Restart(e.Path, e.Version)
}

// ConfigureSelfInstall gives the handler the run's settings for installing
// updates itself. Until it is called the handler never installs. No command
// flag changes these settings, so the run's config is final.
func (h *Handler) ConfigureSelfInstall(config configuration.Config) {
	h.autoUpdate = config.Update.AutoUpdate
	h.installers = map[installmethod.Method]installer{
		installmethod.MethodInstaller: release.Installer{
			BaseURL:    config.Update.ReleaseBaseURL,
			LockDir:    config.Runtime.StateDir,
			Executable: h.executable,
		},
		installmethod.MethodNPM: npm.Installer{
			LockDir:    config.Runtime.StateDir,
			Executable: h.executable,
			Output:     h.out,
		},
	}
}

// installRequired installs the version required asks for and returns a
// RestartError. When it does not install, it returns required, so the update
// instruction and exit 1 apply as they do with auto_update off.
func (h *Handler) installRequired(ctx context.Context, required RequiredError) error {
	log := zerolog.Ctx(ctx)
	method := installmethod.Detect()
	if reason := h.installBlocked(method); reason != "" {
		log.Debug().Str("reason", reason).Msg("self-install skipped")
		return required
	}
	target := installTarget(required)
	_, _ = fmt.Fprintf(h.out, "Updating the TollBit CLI from %s to %s...\n", h.current, target)
	result, err := h.installers[method].Install(ctx, h.current, target)
	if err != nil {
		_, _ = fmt.Fprintf(h.out, "Could not update the TollBit CLI: %v\n", err)
		return required
	}
	_, _ = fmt.Fprintf(h.out, "Updated the TollBit CLI to %s; running the command again.\n", result.Version)
	return RestartError{Path: result.Path, Version: result.Version}
}

// installBlocked returns why this run must not install an update itself, or
// "" when it may.
func (h *Handler) installBlocked(method installmethod.Method) string {
	if os.Getenv(selfinstall.GuardEnvVar) != "" {
		return "already updated once for this command"
	}
	if !h.autoUpdate {
		return "update.auto_update is off"
	}
	// Previews ship from a separate repository; a release from BaseURL
	// would replace a preview with an older public build.
	if current, err := semver.Parse(h.current); err != nil || current.GetPrerelease() != "" {
		return "preview build"
	}
	// go install owns its binary, and an unknown method has no known owner;
	// replacing either behind its back would leave it inconsistent.
	if _, ok := h.installers[method]; !ok {
		return "install method " + string(method)
	}
	return ""
}

// installTarget is the policy's latest version, or its minimum when latest
// is missing or below it.
func installTarget(required RequiredError) string {
	if required.Latest == "" {
		return required.Minimum
	}
	latest, err := semver.Parse(required.Latest)
	if err != nil {
		return required.Minimum
	}
	if minimum, err := semver.Parse(required.Minimum); err == nil && latest.ComparePrecedence(minimum) < 0 {
		return required.Minimum
	}
	return required.Latest
}

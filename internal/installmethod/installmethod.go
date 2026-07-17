// Package installmethod detects how this CLI binary was installed so update
// instructions can name the right command. The backend only reports version
// facts (latest/minimum); remediation is composed here because only the
// client knows its install channel.
package installmethod

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

type Method string

const (
	MethodNPM       Method = "npm"
	MethodInstaller Method = "installer"
	MethodGoInstall Method = "go-install"
	MethodUnknown   Method = "unknown"

	// EnvVar is set by the npm wrapper (npm/cli.js) on every invocation.
	EnvVar = "TOLLBIT_INSTALL_METHOD"
	// MarkerFilename is written next to the binary by the install scripts.
	MarkerFilename = ".tollbit-install-method"
)

// readMainVersion returns the main module's version from the binary's build
// info, or "" if unavailable. GoReleaser, `go build`, and `go test` binaries
// report "" or "(devel)"; only `go install pkg@version` yields a real version.
// It is a var so tests can stub it.
var readMainVersion = func() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return bi.Main.Version
}

// Detect resolves the install method with the following precedence:
// wrapper-provided env var, marker file next to the binary, node_modules
// path heuristic, go-install build-info signal, unknown.
func Detect() Method {
	if m := parse(os.Getenv(EnvVar)); m != MethodUnknown {
		return m
	}

	exe, err := os.Executable()
	if err != nil {
		return MethodUnknown
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	if raw, err := os.ReadFile(filepath.Join(filepath.Dir(exe), MarkerFilename)); err == nil {
		if m := parse(string(raw)); m != MethodUnknown {
			return m
		}
	}

	if strings.Contains(exe, string(filepath.Separator)+"node_modules"+string(filepath.Separator)) {
		return MethodNPM
	}
	if v := readMainVersion(); v != "" && v != "(devel)" {
		return MethodGoInstall
	}
	return MethodUnknown
}

// UpdateInstructions returns a single-line update notice for the given
// method. latest may be empty when the newest version is not known.
func UpdateInstructions(method Method, latest string) string {
	available := "A new version of the TollBit CLI is available"
	if latest != "" {
		available += " (" + latest + ")"
	}

	switch method {
	case MethodNPM:
		return available + ". Run: npm update -g @tollbit/tollbit-cli"
	case MethodInstaller:
		return available + `. Run: curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash -s -- --force`
	case MethodGoInstall:
		return available + ". Run: go install github.com/tollbit/cli/cmd/tollbit@latest"
	default:
		return available + ". See https://github.com/tollbit/cli for install instructions."
	}
}

// RequiredInstructions returns the message for a hard "update required"
// rejection from the backend.
func RequiredInstructions(method Method, minimum, latest string) string {
	msg := "This version of the TollBit CLI is no longer supported"
	if minimum != "" {
		msg += " (minimum: " + minimum + ")"
	}
	msg += ". "

	target := "the latest version"
	if latest != "" {
		target = "version " + latest
	}

	switch method {
	case MethodNPM:
		return msg + "Update to " + target + " with: npm update -g @tollbit/tollbit-cli"
	case MethodInstaller:
		return msg + "Update to " + target + ` with: curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash -s -- --force`
	case MethodGoInstall:
		return msg + "Update to " + target + " with: go install github.com/tollbit/cli/cmd/tollbit@latest"
	default:
		return msg + "Update to " + target + ": see https://github.com/tollbit/cli for install instructions."
	}
}

func parse(raw string) Method {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(MethodNPM):
		return MethodNPM
	case string(MethodInstaller):
		return MethodInstaller
	default:
		return MethodUnknown
	}
}

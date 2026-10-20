// Package selfinstall holds what every way of updating the CLI binary in
// place shares: the no-downgrade check, the lock that keeps two processes
// from installing at once, the check that the new binary runs, and running
// the command again on it. The installs themselves live in the release
// (published release archive) and npm (`npm install -g`) packages.
package selfinstall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/launchdarkly/go-semver"
)

const (
	// GuardEnvVar is set to the installed version on the command that runs
	// again after an install, so each command installs at most once.
	GuardEnvVar = "TOLLBIT_SELF_INSTALLED"
	// LockStale is how old a lock file must be before another process takes
	// it over. An install must finish well within it.
	LockStale = 2 * time.Minute

	versionTimeout = 5 * time.Second
)

var (
	// ErrNotNewer means the target version is not newer than the running
	// one; installs never downgrade.
	ErrNotNewer = errors.New("target version is not newer than the running version")
	// ErrNotWritable means a directory the install must change cannot be
	// written, so the binary cannot be replaced without elevated permissions.
	ErrNotWritable = errors.New("binary directory is not writable")
)

// Result describes the binary to run the command again on.
type Result struct {
	Path    string
	Version string
	// Installed is false when another process had already installed a
	// version at least as new as the target.
	Installed bool
}

// CheckNewer returns ErrNotNewer unless target is newer than current.
func CheckNewer(current, target string) error {
	currentVersion, err := semver.Parse(current)
	if err != nil {
		return fmt.Errorf("parse running version %q: %w", current, err)
	}
	targetVersion, err := semver.Parse(target)
	if err != nil {
		return fmt.Errorf("parse target version %q: %w", target, err)
	}
	if targetVersion.ComparePrecedence(currentVersion) <= 0 {
		return ErrNotNewer
	}
	return nil
}

// Executable returns the binary to replace with symlinks followed, so the
// real file is replaced: exe, or os.Executable() when exe is empty.
func Executable(exe string) (string, error) {
	if exe == "" {
		var err error
		if exe, err = os.Executable(); err != nil {
			return "", fmt.Errorf("find running binary: %w", err)
		}
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve running binary: %w", err)
	}
	return resolved, nil
}

// CheckWritable creates and removes a file in dir, the only portable way to
// learn whether a rename into dir will succeed.
func CheckWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".tollbit-write-check-*")
	if err != nil {
		return fmt.Errorf("%w: %s", ErrNotWritable, dir)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}

// InstallLocked runs install to put version target at exe while holding the
// lock in lockDir. It skips install when the binary on disk is already at
// least target, which happens when another process installed while this one
// waited for the lock, and checks afterwards that exe runs and reports
// target.
func InstallLocked(ctx context.Context, lockDir, exe, target string, install func(context.Context) error) (Result, error) {
	targetVersion, err := semver.Parse(target)
	if err != nil {
		return Result{}, fmt.Errorf("parse target version %q: %w", target, err)
	}
	unlock, err := acquireLock(ctx, filepath.Join(lockDir, lockFilename), lockWait, LockStale)
	if err != nil {
		return Result{}, err
	}
	defer unlock()

	if onDisk, raw, ok := installedVersion(ctx, exe); ok && onDisk.ComparePrecedence(targetVersion) >= 0 {
		return Result{Path: exe, Version: raw}, nil
	}
	if err := install(ctx); err != nil {
		return Result{}, err
	}
	if onDisk, raw, ok := installedVersion(ctx, exe); !ok || onDisk.ComparePrecedence(targetVersion) < 0 {
		return Result{}, fmt.Errorf("installed %s but %s reports version %q", target, exe, raw)
	}
	return Result{Path: exe, Version: target, Installed: true}, nil
}

// installedVersion runs `<exe> version` and reports the version it prints,
// parsed and as printed.
func installedVersion(ctx context.Context, exe string) (semver.Version, string, bool) {
	ctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, exe, "version").Output()
	if err != nil {
		return semver.Version{}, "", false
	}
	raw := strings.TrimSpace(string(out))
	v, err := semver.Parse(raw)
	if err != nil {
		return semver.Version{}, raw, false
	}
	return v, raw, true
}

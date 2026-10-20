package selfinstall

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tollbit/cli/internal/selfinstall/selfinstalltest"
	"github.com/tollbit/cli/internal/selfinstall/selfinstalltest/fakecli"
)

type lockedEnv struct {
	lockDir string
	exe     string
	// installs counts calls of install.
	installs int
}

// newLockedEnv puts a fake binary at current in a temp dir.
func newLockedEnv(t *testing.T, current string) *lockedEnv {
	t.Helper()
	exe := filepath.Join(t.TempDir(), selfinstalltest.ExeName())
	selfinstalltest.WriteBinary(t, exe, current)
	return &lockedEnv{lockDir: t.TempDir(), exe: exe}
}

// install returns an install func that writes the version binary to exe.
func (e *lockedEnv) install(t *testing.T, version string) func(context.Context) error {
	return func(context.Context) error {
		e.installs++
		selfinstalltest.WriteBinary(t, e.exe, version)
		return nil
	}
}

func (e *lockedEnv) holdLock(t *testing.T) string {
	t.Helper()
	lock := filepath.Join(e.lockDir, lockFilename)
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return lock
}

func shortenLockWait(t *testing.T) {
	t.Helper()
	saved := lockWait
	lockWait = 2 * lockPoll
	t.Cleanup(func() { lockWait = saved })
}

func TestInstallLockedInstallsAndReleasesLock(t *testing.T) {
	env := newLockedEnv(t, "0.1.0")

	result, err := InstallLocked(context.Background(), env.lockDir, env.exe, "0.2.0", env.install(t, "0.2.0"))
	if err != nil {
		t.Fatal(err)
	}

	if result != (Result{Path: env.exe, Version: "0.2.0", Installed: true}) {
		t.Fatalf("unexpected result %+v", result)
	}
	selfinstalltest.AssertBinary(t, env.exe, "0.2.0")
	if _, err := os.Stat(filepath.Join(env.lockDir, lockFilename)); !os.IsNotExist(err) {
		t.Fatalf("expected the lock to be released, got %v", err)
	}
}

func TestInstallLockedFailsWhenBinaryDoesNotReportTarget(t *testing.T) {
	env := newLockedEnv(t, "0.1.0")

	_, err := InstallLocked(context.Background(), env.lockDir, env.exe, "0.2.0", env.install(t, "0.1.5"))

	if err == nil || !strings.Contains(err.Error(), `reports version "0.1.5"`) {
		t.Fatalf("expected the version check to fail, got %v", err)
	}
}

func TestInstallLockedPassesOnInstallError(t *testing.T) {
	env := newLockedEnv(t, "0.1.0")
	boom := errors.New("boom")

	_, err := InstallLocked(context.Background(), env.lockDir, env.exe, "0.2.0", func(context.Context) error { return boom })

	if !errors.Is(err, boom) {
		t.Fatalf("expected the install error, got %v", err)
	}
	selfinstalltest.AssertBinary(t, env.exe, "0.1.0")
}

func TestInstallLockedAfterWaitingForLockUsesVersionAlreadyInstalled(t *testing.T) {
	env := newLockedEnv(t, "0.1.0")
	lock := env.holdLock(t)
	type outcome struct {
		result Result
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := InstallLocked(context.Background(), env.lockDir, env.exe, "0.2.0", env.install(t, "0.2.0"))
		done <- outcome{result, err}
	}()
	// Another process finishes installing 0.2.0, then releases the lock.
	time.Sleep(2 * lockPoll)
	selfinstalltest.WriteBinary(t, env.exe, "0.2.0")
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	got := <-done

	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.result != (Result{Path: env.exe, Version: "0.2.0"}) {
		t.Fatalf("unexpected result %+v", got.result)
	}
	if env.installs != 0 {
		t.Fatalf("expected no install, got %d", env.installs)
	}
}

func TestInstallLockedGivesUpWhileLockStaysHeld(t *testing.T) {
	env := newLockedEnv(t, "0.1.0")
	env.holdLock(t)
	shortenLockWait(t)

	_, err := InstallLocked(context.Background(), env.lockDir, env.exe, "0.2.0", env.install(t, "0.2.0"))

	if err == nil || !strings.Contains(err.Error(), "in progress") {
		t.Fatalf("expected the held lock to stop the install, got %v", err)
	}
	if env.installs != 0 {
		t.Fatalf("expected no install, got %d", env.installs)
	}
	selfinstalltest.AssertBinary(t, env.exe, "0.1.0")
}

func TestInstallLockedTakesOverStaleLock(t *testing.T) {
	env := newLockedEnv(t, "0.1.0")
	lock := env.holdLock(t)
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	shortenLockWait(t)

	if _, err := InstallLocked(context.Background(), env.lockDir, env.exe, "0.2.0", env.install(t, "0.2.0")); err != nil {
		t.Fatal(err)
	}
	selfinstalltest.AssertBinary(t, env.exe, "0.2.0")
}

// TestRestartHelper is the process TestRestartRunsCommandAgain restarts.
// It exits with Restart's code the way main does; on macOS and Linux Restart
// returns only on failure.
func TestRestartHelper(t *testing.T) {
	target := os.Getenv("SELFINSTALL_TEST_RESTART_TARGET")
	if target == "" {
		t.Skip("run by TestRestartRunsCommandAgain")
	}
	code, err := Restart(target, "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	os.Exit(code)
}

func TestRestartRunsCommandAgain(t *testing.T) {
	target := filepath.Join(t.TempDir(), selfinstalltest.ExeName())
	selfinstalltest.WriteBinary(t, target, "0.2.0")
	cmd := exec.Command(os.Args[0], "-test.run=^TestRestartHelper$", "search", "q")
	cmd.Env = append(os.Environ(), "SELFINSTALL_TEST_RESTART_TARGET="+target)

	out, err := cmd.Output()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != fakecli.RunExitCode {
		t.Fatalf("expected the restarted binary's exit code %d, got %v (output %q)", fakecli.RunExitCode, err, out)
	}
	if !strings.Contains(string(out), "args: -test.run=^TestRestartHelper$ search q\n") {
		t.Fatalf("expected the same arguments, got %q", out)
	}
	if !strings.Contains(string(out), GuardEnvVar+"=0.2.0\n") {
		t.Fatalf("expected the guard env var, got %q", out)
	}
}

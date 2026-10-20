package release

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/tollbit/cli/internal/selfinstall"
	"github.com/tollbit/cli/internal/selfinstall/selfinstalltest"
)

// On macOS and Linux, the Windows steps also run here with goos set to
// "windows".

type installEnv struct {
	installer Installer
	exe       string
	server    selfinstalltest.Server
}

// newInstallEnv puts a fake binary at current in a temp dir and serves
// release from a fake release server.
func newInstallEnv(t *testing.T, current string, release selfinstalltest.Release) installEnv {
	t.Helper()
	name := selfinstalltest.ExeName()
	if release.GOOS == "windows" {
		name = "tollbit.exe"
	}
	// Resolved, because the installer reports the path with symlinks
	// followed (macOS temp dirs sit behind one).
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, name)
	selfinstalltest.WriteBinary(t, exe, current)
	server := selfinstalltest.Serve(t, release)
	return installEnv{
		installer: Installer{
			BaseURL:    server.URL,
			LockDir:    t.TempDir(),
			Executable: exe,
			goos:       release.GOOS,
		},
		exe:    exe,
		server: server,
	}
}

func TestInstallReplacesBinaryWithVerifiedRelease(t *testing.T) {
	env := newInstallEnv(t, "0.1.0", selfinstalltest.Release{Version: "0.2.0"})

	result, err := env.installer.Install(context.Background(), "0.1.0", "0.2.0")
	if err != nil {
		t.Fatal(err)
	}

	if result != (selfinstall.Result{Path: env.exe, Version: "0.2.0", Installed: true}) {
		t.Fatalf("unexpected result %+v", result)
	}
	selfinstalltest.AssertBinary(t, env.exe, "0.2.0")
	// Windows has no executable bit; it runs any file named .exe.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(env.exe)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("expected an executable, got mode %s", info.Mode())
		}
	}
	want := []string{filepath.Base(env.exe)}
	if runtime.GOOS == "windows" {
		want = append(want, filepath.Base(env.exe)+oldSuffix)
	}
	if got := dirNames(t, filepath.Dir(env.exe)); !slices.Equal(got, want) {
		t.Fatalf("expected only %v to remain, got %v", want, got)
	}
}

// TestInstallReplacesRunningBinary replaces the binary while a process runs
// from it, as the CLI does. Windows refuses to overwrite a running binary, so
// there this runs the move to exe.old for real.
func TestInstallReplacesRunningBinary(t *testing.T) {
	env := newInstallEnv(t, "0.1.0", selfinstalltest.Release{Version: "0.2.0"})
	running := exec.Command(env.exe, "wait")
	stdin, err := running.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := running.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = running.Wait()
	})

	if _, err := env.installer.Install(context.Background(), "0.1.0", "0.2.0"); err != nil {
		t.Fatal(err)
	}

	selfinstalltest.AssertBinary(t, env.exe, "0.2.0")
	if runtime.GOOS == "windows" {
		selfinstalltest.AssertBinary(t, env.exe+oldSuffix, "0.1.0")
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := running.Wait(); err != nil {
		t.Fatalf("expected the running binary to finish normally, got %v", err)
	}
}

func TestInstallReplacesSymlinkTarget(t *testing.T) {
	env := newInstallEnv(t, "0.1.0", selfinstalltest.Release{Version: "0.2.0"})
	link := filepath.Join(t.TempDir(), filepath.Base(env.exe))
	if err := os.Symlink(env.exe, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("creating a symlink needs Developer Mode or an administrator on Windows: %v", err)
		}
		t.Fatal(err)
	}
	env.installer.Executable = link

	result, err := env.installer.Install(context.Background(), "0.1.0", "0.2.0")
	if err != nil {
		t.Fatal(err)
	}

	if result.Path != env.exe {
		t.Fatalf("expected the symlink target %s, got %s", env.exe, result.Path)
	}
	selfinstalltest.AssertBinary(t, env.exe, "0.2.0")
	if target, err := os.Readlink(link); err != nil || target != env.exe {
		t.Fatalf("expected the symlink to be kept, got %q %v", target, err)
	}
}

func TestInstallStopsOnChecksumMismatch(t *testing.T) {
	env := newInstallEnv(t, "0.1.0", selfinstalltest.Release{Version: "0.2.0", BadChecksum: true})

	_, err := env.installer.Install(context.Background(), "0.1.0", "0.2.0")

	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected a checksum mismatch, got %v", err)
	}
	selfinstalltest.AssertBinary(t, env.exe, "0.1.0")
	if got := dirNames(t, filepath.Dir(env.exe)); !slices.Equal(got, []string{filepath.Base(env.exe)}) {
		t.Fatalf("expected only the binary to remain, got %v", got)
	}
}

func TestInstallRefusesVersionNotAboveRunning(t *testing.T) {
	for _, target := range []string{"0.1.0", "0.2.0"} {
		t.Run(target, func(t *testing.T) {
			env := newInstallEnv(t, "0.2.0", selfinstalltest.Release{Version: target})

			_, err := env.installer.Install(context.Background(), "0.2.0", target)

			if !errors.Is(err, selfinstall.ErrNotNewer) {
				t.Fatalf("expected ErrNotNewer, got %v", err)
			}
			if got := env.server.Requests.Load(); got != 0 {
				t.Fatalf("expected no downloads, got %d requests", got)
			}
			selfinstalltest.AssertBinary(t, env.exe, "0.2.0")
		})
	}
}

func TestInstallRefusesNonWritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write any directory")
	}
	if runtime.GOOS == "windows" {
		t.Skip("Windows ignores the read-only attribute on directories")
	}
	env := newInstallEnv(t, "0.1.0", selfinstalltest.Release{Version: "0.2.0"})
	dir := filepath.Dir(env.exe)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := env.installer.Install(context.Background(), "0.1.0", "0.2.0")

	if !errors.Is(err, selfinstall.ErrNotWritable) {
		t.Fatalf("expected ErrNotWritable, got %v", err)
	}
	if got := env.server.Requests.Load(); got != 0 {
		t.Fatalf("expected no downloads, got %d requests", got)
	}
}

func TestInstallOnWindowsMovesRunningBinaryAside(t *testing.T) {
	env := newInstallEnv(t, "0.1.0", selfinstalltest.Release{Version: "0.2.0", GOOS: "windows"})

	if _, err := env.installer.Install(context.Background(), "0.1.0", "0.2.0"); err != nil {
		t.Fatal(err)
	}

	selfinstalltest.AssertBinary(t, env.exe, "0.2.0")
	selfinstalltest.AssertBinary(t, env.exe+oldSuffix, "0.1.0")
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

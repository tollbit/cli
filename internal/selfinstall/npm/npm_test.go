package npm

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/tollbit/cli/internal/selfinstall"
	"github.com/tollbit/cli/internal/selfinstall/selfinstalltest"
)

const npmInstallCall = "install -g @tollbit/tollbit-cli@0.3.0"

// newNPMEnv fakes a global npm install of 0.1.0 whose npm install fails when
// fail is set.
func newNPMEnv(t *testing.T, fail bool) (Installer, selfinstalltest.NPMInstall, *bytes.Buffer) {
	t.Helper()
	npm := selfinstalltest.FakeNPM(t, "0.1.0", fail)
	out := new(bytes.Buffer)
	return Installer{LockDir: t.TempDir(), Executable: npm.Exe, Output: out}, npm, out
}

// skipOnWindows skips a test of what Install does before or after running
// npm: on Windows it refuses first, which TestInstallRefuses/Windows covers.
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Install refuses on Windows before it runs npm")
	}
}

// writeBinaryAt puts a 0.1.0 binary at rel under a new temp dir and returns
// its path.
func writeBinaryAt(t *testing.T, rel string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	selfinstalltest.WriteBinary(t, exe, "0.1.0")
	return exe
}

func TestInstallInstallsGlobalPackage(t *testing.T) {
	skipOnWindows(t)
	installer, npm, out := newNPMEnv(t, false)

	result, err := installer.Install(context.Background(), "0.1.0", "0.3.0")
	if err != nil {
		t.Fatal(err)
	}

	if result != (selfinstall.Result{Path: npm.Exe, Version: "0.3.0", Installed: true}) {
		t.Fatalf("unexpected result %+v", result)
	}
	if calls := npm.NPMCalls(t); !slices.Equal(calls, []string{"prefix -g", npmInstallCall}) {
		t.Fatalf("unexpected npm calls %q", calls)
	}
	if !strings.Contains(out.String(), "changed 1 package") {
		t.Fatalf("expected npm's output, got %q", out.String())
	}
	selfinstalltest.AssertBinary(t, npm.Exe, "0.3.0")
}

func TestInstallReportsFailedInstall(t *testing.T) {
	skipOnWindows(t)
	installer, npm, out := newNPMEnv(t, true)

	_, err := installer.Install(context.Background(), "0.1.0", "0.3.0")

	if err == nil || !strings.Contains(err.Error(), "npm "+npmInstallCall) {
		t.Fatalf("expected the npm install error, got %v", err)
	}
	if !strings.Contains(out.String(), "npm error code E404") {
		t.Fatalf("expected npm's output, got %q", out.String())
	}
	selfinstalltest.AssertBinary(t, npm.Exe, "0.1.0")
}

func TestInstallUsesVersionAlreadyInstalled(t *testing.T) {
	skipOnWindows(t)
	installer, npm, _ := newNPMEnv(t, false)
	// Another process installed 0.3.0 since this one started.
	selfinstalltest.WriteBinary(t, npm.Exe, "0.3.0")

	result, err := installer.Install(context.Background(), "0.1.0", "0.3.0")
	if err != nil {
		t.Fatal(err)
	}

	if result.Installed || result.Version != "0.3.0" {
		t.Fatalf("expected the installed 0.3.0, got %+v", result)
	}
	if calls := npm.NPMCalls(t); slices.Contains(calls, npmInstallCall) {
		t.Fatalf("expected no npm install, got %q", calls)
	}
}

func TestInstallRefuses(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, installer *Installer, npm selfinstalltest.NPMInstall)
		want  error
	}{
		{
			name:  "Windows",
			setup: func(_ *testing.T, installer *Installer, _ selfinstalltest.NPMInstall) { installer.goos = "windows" },
			want:  ErrWindows,
		},
		{
			name:  "npm not on PATH",
			setup: func(t *testing.T, _ *Installer, _ selfinstalltest.NPMInstall) { t.Setenv("PATH", t.TempDir()) },
			want:  ErrNotFound,
		},
		{
			name: "project node_modules",
			setup: func(t *testing.T, installer *Installer, _ selfinstalltest.NPMInstall) {
				installer.Executable = writeBinaryAt(t, "project/node_modules/@tollbit/tollbit-cli/tollbit")
			},
			want: ErrNotGlobal,
		},
		{
			name: "npx",
			setup: func(t *testing.T, installer *Installer, _ selfinstalltest.NPMInstall) {
				installer.Executable = writeBinaryAt(t, ".npm/_npx/1a2b/node_modules/@tollbit/tollbit-cli/tollbit")
			},
			want: ErrNotGlobal,
		},
		{
			name: "pnpm",
			setup: func(t *testing.T, installer *Installer, _ selfinstalltest.NPMInstall) {
				installer.Executable = writeBinaryAt(t, "Library/pnpm/global/5/.pnpm/@tollbit+tollbit-cli@0.1.0/node_modules/@tollbit/tollbit-cli/tollbit")
			},
			want: ErrNotGlobal,
		},
		{
			name: "Yarn",
			setup: func(t *testing.T, installer *Installer, _ selfinstalltest.NPMInstall) {
				installer.Executable = writeBinaryAt(t, ".config/yarn/global/node_modules/@tollbit/tollbit-cli/tollbit")
			},
			want: ErrNotGlobal,
		},
		{
			name: "Volta",
			setup: func(t *testing.T, installer *Installer, _ selfinstalltest.NPMInstall) {
				installer.Executable = writeBinaryAt(t, ".volta/tools/image/packages/@tollbit/tollbit-cli/lib/node_modules/@tollbit/tollbit-cli/tollbit")
			},
			want: ErrNotGlobal,
		},
		{
			name: "global folder not writable",
			setup: func(t *testing.T, _ *Installer, npm selfinstalltest.NPMInstall) {
				if os.Geteuid() == 0 {
					t.Skip("root can write any directory")
				}
				dir := filepath.Join(npm.Prefix, "lib", "node_modules", "@tollbit")
				if err := os.Chmod(dir, 0o555); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			},
			want: selfinstall.ErrNotWritable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.want, ErrWindows) {
				skipOnWindows(t)
			}
			installer, npm, _ := newNPMEnv(t, false)
			tt.setup(t, &installer, npm)

			_, err := installer.Install(context.Background(), "0.1.0", "0.3.0")

			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
			if calls := npm.NPMCalls(t); slices.Contains(calls, npmInstallCall) {
				t.Fatalf("expected no npm install, got %q", calls)
			}
			selfinstalltest.AssertBinary(t, npm.Exe, "0.1.0")
		})
	}
}

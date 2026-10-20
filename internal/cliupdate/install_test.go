package cliupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tollbit/cli/internal/client/versionpolicy"
	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/errorsx/problemjson"
	"github.com/tollbit/cli/internal/installmethod"
	"github.com/tollbit/cli/internal/selfinstall"
	"github.com/tollbit/cli/internal/selfinstall/selfinstalltest"
)

const (
	runningVersion = "0.1.0"
	minimumVersion = "0.2.0"
	latestVersion  = "0.3.0"
)

type installEnv struct {
	config  configuration.Config
	policy  versionpolicy.Policy
	handler *Handler
	out     *bytes.Buffer
	exe     string
	release selfinstalltest.Server
}

// newInstallEnv runs runningVersion as an installer build with auto_update
// on, serves release from a fake release server, and reports policy as the
// version policy.
func newInstallEnv(t *testing.T, release selfinstalltest.Release, policy versionpolicy.Policy) installEnv {
	t.Helper()
	t.Setenv(installmethod.EnvVar, string(installmethod.MethodInstaller))

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, selfinstalltest.ExeName())
	selfinstalltest.WriteBinary(t, exe, runningVersion)
	server := selfinstalltest.Serve(t, release)

	var config configuration.Config
	config.Runtime.StateDir = t.TempDir()
	config.Update.AutoUpdate = true
	config.Update.ReleaseBaseURL = server.URL

	out := new(bytes.Buffer)
	handler := NewHandler(out)
	handler.current = runningVersion
	handler.executable = exe
	return installEnv{config: config, policy: policy, handler: handler, out: out, exe: exe, release: server}
}

func belowMinimumPolicy() versionpolicy.Policy {
	return versionpolicy.Policy{Minimum: minimumVersion, Latest: latestVersion}
}

// check runs the update check with the env's settings.
func (e installEnv) check(t *testing.T) error {
	t.Helper()
	e.handler.ConfigureSelfInstall(e.config)
	return NewChecker(t.TempDir(), &fakePolicyClient{policy: e.policy}, time.Hour, e.handler).Run(context.Background())
}

func (e installEnv) assertBinary(t *testing.T, version string) {
	t.Helper()
	selfinstalltest.AssertBinary(t, e.exe, version)
}

func TestCheckBelowMinimumInstallsLatestAndAsksForRestart(t *testing.T) {
	env := newInstallEnv(t, selfinstalltest.Release{Version: latestVersion}, belowMinimumPolicy())

	err := env.check(t)

	var restart RestartError
	if !errors.As(err, &restart) {
		t.Fatalf("expected RestartError, got %v", err)
	}
	if restart != (RestartError{Path: env.exe, Version: latestVersion}) {
		t.Fatalf("unexpected restart %+v", restart)
	}
	env.assertBinary(t, latestVersion)
	want := "Updating the TollBit CLI from 0.1.0 to 0.3.0...\nUpdated the TollBit CLI to 0.3.0; running the command again.\n"
	if env.out.String() != want {
		t.Fatalf("expected progress %q, got %q", want, env.out.String())
	}
}

func TestCheckInstallsMinimumWhenLatestIsMissing(t *testing.T) {
	env := newInstallEnv(t, selfinstalltest.Release{Version: minimumVersion}, versionpolicy.Policy{Minimum: minimumVersion})

	var restart RestartError
	if err := env.check(t); !errors.As(err, &restart) || restart.Version != minimumVersion {
		t.Fatalf("expected a restart on %s, got %v", minimumVersion, err)
	}
	env.assertBinary(t, minimumVersion)
}

func TestCheckBelowMinimumStopsWithoutInstalling(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, env *installEnv)
	}{
		{
			name:  "auto update off",
			setup: func(t *testing.T, env *installEnv) { env.config.Update.AutoUpdate = false },
		},
		{
			name:  "preview build",
			setup: func(t *testing.T, env *installEnv) { env.handler.current = runningVersion + "-preview.1" },
		},
		{
			name: "unknown install method",
			setup: func(t *testing.T, env *installEnv) {
				t.Setenv(installmethod.EnvVar, "")
			},
		},
		{
			name: "already updated for this command",
			setup: func(t *testing.T, env *installEnv) {
				t.Setenv(selfinstall.GuardEnvVar, latestVersion)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newInstallEnv(t, selfinstalltest.Release{Version: latestVersion}, belowMinimumPolicy())
			tt.setup(t, &env)

			err := env.check(t)

			var required RequiredError
			if !errors.As(err, &required) {
				t.Fatalf("expected RequiredError, got %v", err)
			}
			if got := env.release.Requests.Load(); got != 0 {
				t.Fatalf("expected no downloads, got %d requests", got)
			}
			if env.out.Len() != 0 {
				t.Fatalf("expected no progress, got %q", env.out.String())
			}
			env.assertBinary(t, runningVersion)
		})
	}
}

func TestCheckBelowLatestWithAutoUpdateOffPrintsNotice(t *testing.T) {
	env := newInstallEnv(t, selfinstalltest.Release{Version: latestVersion}, versionpolicy.Policy{Latest: latestVersion})
	env.config.Update.AutoUpdate = false

	if err := env.check(t); err != nil {
		t.Fatal(err)
	}

	want := installmethod.UpdateInstructions(installmethod.MethodInstaller, latestVersion) + "\n"
	if env.out.String() != want {
		t.Fatalf("expected notice %q, got %q", want, env.out.String())
	}
	if got := env.release.Requests.Load(); got != 0 {
		t.Fatalf("expected no downloads, got %d requests", got)
	}
}

func TestCheckFailedInstallReportsAndStops(t *testing.T) {
	tests := []struct {
		name    string
		release selfinstalltest.Release
		setup   func(t *testing.T, env *installEnv)
		reason  string
	}{
		{
			name:    "bad checksum",
			release: selfinstalltest.Release{Version: latestVersion, BadChecksum: true},
			setup:   func(*testing.T, *installEnv) {},
			reason:  "checksum mismatch",
		},
		{
			name:    "binary directory not writable",
			release: selfinstalltest.Release{Version: latestVersion},
			setup: func(t *testing.T, env *installEnv) {
				if os.Geteuid() == 0 {
					t.Skip("root can write any directory")
				}
				if runtime.GOOS == "windows" {
					t.Skip("Windows ignores the read-only attribute on directories")
				}
				dir := filepath.Dir(env.exe)
				if err := os.Chmod(dir, 0o555); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			},
			reason: "binary directory is not writable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newInstallEnv(t, tt.release, belowMinimumPolicy())
			tt.setup(t, &env)

			err := env.check(t)

			var required RequiredError
			if !errors.As(err, &required) {
				t.Fatalf("expected RequiredError, got %v", err)
			}
			want := "Updating the TollBit CLI from 0.1.0 to 0.3.0...\nCould not update the TollBit CLI: " + tt.reason
			if !strings.HasPrefix(env.out.String(), want) {
				t.Fatalf("expected stderr to start with %q, got %q", want, env.out.String())
			}
			env.assertBinary(t, runningVersion)
		})
	}
}

// skipNPMOnWindows skips a test of an npm update that gets as far as running
// npm. On Windows the npm installer refuses first, which
// TestCheckNPMUpdateOnWindowsReportsAndStops covers.
func skipNPMOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the npm installer refuses on Windows before it runs npm")
	}
}

// newNPMInstallEnv is newInstallEnv for a global npm install whose npm
// install fails when fail is set.
func newNPMInstallEnv(t *testing.T, fail bool) (installEnv, selfinstalltest.NPMInstall) {
	t.Helper()
	env := newInstallEnv(t, selfinstalltest.Release{Version: latestVersion}, belowMinimumPolicy())
	t.Setenv(installmethod.EnvVar, string(installmethod.MethodNPM))
	npm := selfinstalltest.FakeNPM(t, runningVersion, fail)
	env.exe = npm.Exe
	env.handler.executable = npm.Exe
	return env, npm
}

func TestCheckBelowMinimumUpdatesThroughNPMAndAsksForRestart(t *testing.T) {
	skipNPMOnWindows(t)
	env, npm := newNPMInstallEnv(t, false)

	err := env.check(t)

	var restart RestartError
	if !errors.As(err, &restart) {
		t.Fatalf("expected RestartError, got %v", err)
	}
	if restart != (RestartError{Path: npm.Exe, Version: latestVersion}) {
		t.Fatalf("unexpected restart %+v", restart)
	}
	if calls := npm.NPMCalls(t); calls[len(calls)-1] != "install -g @tollbit/tollbit-cli@"+latestVersion {
		t.Fatalf("expected npm install -g of %s, got %q", latestVersion, calls)
	}
	env.assertBinary(t, latestVersion)
	want := "Updating the TollBit CLI from 0.1.0 to 0.3.0...\nchanged 1 package in 1s\nUpdated the TollBit CLI to 0.3.0; running the command again.\n"
	if env.out.String() != want {
		t.Fatalf("expected progress %q, got %q", want, env.out.String())
	}
	if got := env.release.Requests.Load(); got != 0 {
		t.Fatalf("expected no release downloads, got %d requests", got)
	}
}

func TestCheckFailedNPMUpdateReportsAndStops(t *testing.T) {
	skipNPMOnWindows(t)
	tests := []struct {
		name   string
		fail   bool
		setup  func(t *testing.T, env *installEnv)
		reason string
	}{
		{
			name:   "npm install fails",
			fail:   true,
			setup:  func(*testing.T, *installEnv) {},
			reason: "npm error code E404\nCould not update the TollBit CLI: npm install -g @tollbit/tollbit-cli@0.3.0: exit status 1",
		},
		{
			name:   "npm not on PATH",
			setup:  func(t *testing.T, _ *installEnv) { t.Setenv("PATH", t.TempDir()) },
			reason: "Could not update the TollBit CLI: npm is not on PATH",
		},
		{
			name: "not a global install",
			setup: func(t *testing.T, env *installEnv) {
				exe := filepath.Join(t.TempDir(), "node_modules", "@tollbit", "tollbit-cli", "tollbit")
				if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
					t.Fatal(err)
				}
				selfinstalltest.WriteBinary(t, exe, runningVersion)
				env.handler.executable = exe
			},
			reason: "Could not update the TollBit CLI: not a global npm install",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, _ := newNPMInstallEnv(t, tt.fail)
			tt.setup(t, &env)

			err := env.check(t)

			var required RequiredError
			if !errors.As(err, &required) {
				t.Fatalf("expected RequiredError, got %v", err)
			}
			if !strings.Contains(err.Error(), "npm update -g @tollbit/tollbit-cli") {
				t.Fatalf("expected the npm instruction, got %q", err.Error())
			}
			want := "Updating the TollBit CLI from 0.1.0 to 0.3.0...\n" + tt.reason
			if !strings.HasPrefix(env.out.String(), want) {
				t.Fatalf("expected stderr to start with %q, got %q", want, env.out.String())
			}
			env.assertBinary(t, runningVersion)
		})
	}
}

// updateRequiredProblem is the gateway's 426 response body as the API
// clients return it.
func updateRequiredProblem(t *testing.T) error {
	t.Helper()
	problem, err := problemjson.Parse([]byte(fmt.Sprintf(
		`{"title":"CLI update required","status":426,"code":"cli_update_required","minimumVersion":%q,"latestVersion":%q}`,
		minimumVersion, latestVersion)))
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Errorf("error searching: %w", problem)
}

func TestUpdateRequiredResponseMakesNextCheckInstall(t *testing.T) {
	env := newInstallEnv(t, selfinstalltest.Release{Version: latestVersion}, versionpolicy.Policy{})
	env.handler.ConfigureSelfInstall(env.config)
	// The policy endpoint has not caught up with the gateway yet.
	client := &fakePolicyClient{}
	checker := NewChecker(t.TempDir(), client, time.Hour, env.handler)

	checker.RecordUpdateRequired(context.Background(), updateRequiredProblem(t))
	err := checker.Run(context.Background())

	var restart RestartError
	if !errors.As(err, &restart) || restart.Version != latestVersion {
		t.Fatalf("expected a restart on %s, got %v", latestVersion, err)
	}
	if client.calls != 0 {
		t.Fatalf("expected the recorded versions to be used, got %d policy requests", client.calls)
	}
	env.assertBinary(t, latestVersion)
}

func TestOtherCommandErrorsAreNotRecorded(t *testing.T) {
	env := newInstallEnv(t, selfinstalltest.Release{Version: latestVersion}, versionpolicy.Policy{})
	env.handler.ConfigureSelfInstall(env.config)
	client := &fakePolicyClient{}
	checker := NewChecker(t.TempDir(), client, time.Hour, env.handler)

	checker.RecordUpdateRequired(context.Background(), errors.New("boom"))
	if err := checker.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if client.calls != 1 {
		t.Fatalf("expected the empty cache to be fetched, got %d policy requests", client.calls)
	}
	if got := env.release.Requests.Load(); got != 0 {
		t.Fatalf("expected no downloads, got %d requests", got)
	}
	env.assertBinary(t, runningVersion)
}

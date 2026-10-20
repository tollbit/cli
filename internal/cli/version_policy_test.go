package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/launchdarkly/go-semver"
	"github.com/tollbit/cli/internal/app"
	"github.com/tollbit/cli/internal/client/tollbit"
	"github.com/tollbit/cli/internal/client/versionpolicy"
	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/installmethod"
	"github.com/tollbit/cli/internal/version"
)

// These tests run the whole command tree, because what they prove is the
// wiring: the root command's update check and the transport on the API
// clients share the run's Process, and errors render the way main renders
// them. The checking logic itself is tested in cliupdate.

type (
	versionPolicyEnv struct {
		config   configuration.Config
		requests *atomic.Int32
	}

	versionPolicyResult struct {
		code   int
		stdout string
		stderr string
	}
)

// newVersionPolicyEnv serves policy from a fake Foundry and enables the check
// with a one-hour interval. A nil policy makes the endpoint fail.
func newVersionPolicyEnv(t *testing.T, policy *versionpolicy.Policy) versionPolicyEnv {
	t.Helper()
	t.Setenv(installmethod.EnvVar, string(installmethod.MethodNPM))

	requests := new(atomic.Int32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/foundry/cli/v1/version-policy" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if policy == nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"minimum":%q,"latest":%q}`, policy.Minimum, policy.Latest)
	}))
	t.Cleanup(srv.Close)

	config := testConfig()
	config.Runtime.StateDir = t.TempDir()
	config.Foundry.BaseURL = srv.URL + "/foundry"
	config.Update.CheckEnabled = true
	config.Update.CheckInterval = time.Hour
	return versionPolicyEnv{config: config, requests: requests}
}

// fakeGatewaySearch serves an auth token and an empty search page whose
// response carries latestHeader as X-Tollbit-Cli-Latest-Version.
func (e *versionPolicyEnv) fakeGatewaySearch(t *testing.T, latestHeader string) {
	t.Helper()
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	t.Cleanup(authSrv.Close)
	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Tollbit-Cli-Latest-Version", latestHeader)
		_ = json.NewEncoder(w).Encode(tollbit.PagedSearchResultResponse{})
	}))
	t.Cleanup(gatewaySrv.Close)
	e.config.Auth.BaseURL = authSrv.URL
	e.config.Gateway.BaseURL = gatewaySrv.URL
	e.config.Credentials.StorageDir = t.TempDir()
}

// run executes args the way main does, rendering any error on stderr.
func (e versionPolicyEnv) run(t *testing.T, args ...string) versionPolicyResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := NewCommandTree(app.NewFactory(e.config, &stderr))
	cmd.SetArgs(args)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err != nil {
		fmt.Fprintln(&stderr, RenderError(err))
	}
	return versionPolicyResult{code: ExitCode(err), stdout: stdout.String(), stderr: stderr.String()}
}

// versionAbove returns a version newer than this binary.
func versionAbove(t *testing.T) string {
	t.Helper()
	current, err := semver.Parse(version.String())
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%d.%d.0", current.GetMajor(), current.GetMinor()+1)
}

func TestVersionPolicyBelowMinimumFailsWithInstruction(t *testing.T) {
	minimum := versionAbove(t)
	env := newVersionPolicyEnv(t, &versionpolicy.Policy{Minimum: minimum, Latest: minimum})

	got := env.run(t, "runtime", "status")

	if got.code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", got.code, got.stderr)
	}
	if got.stdout != "" {
		t.Fatalf("expected no stdout, got %q", got.stdout)
	}
	want := installmethod.RequiredInstructions(installmethod.MethodNPM, minimum, minimum)
	if strings.TrimSpace(got.stderr) != want {
		t.Fatalf("expected stderr %q, got %q", want, got.stderr)
	}
}

func TestVersionPolicyGatewayHeaderPrintsNotice(t *testing.T) {
	latest := versionAbove(t)
	env := newVersionPolicyEnv(t, &versionpolicy.Policy{})
	env.config.Update.CheckEnabled = false
	env.fakeGatewaySearch(t, latest)

	got := env.run(t, "search", "q")

	if got.code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", got.code, got.stderr)
	}
	want := installmethod.UpdateInstructions(installmethod.MethodNPM, latest)
	if !strings.Contains(got.stderr, want) {
		t.Fatalf("expected notice %q on stderr, got %q", want, got.stderr)
	}
	if strings.Contains(got.stdout, want) {
		t.Fatalf("notice leaked to stdout: %q", got.stdout)
	}
}

func TestVersionPolicyNoticePrintedOncePerRun(t *testing.T) {
	latest := versionAbove(t)
	env := newVersionPolicyEnv(t, &versionpolicy.Policy{Latest: latest})
	env.fakeGatewaySearch(t, latest)

	got := env.run(t, "search", "q")

	if got.code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", got.code, got.stderr)
	}
	want := installmethod.UpdateInstructions(installmethod.MethodNPM, latest)
	if n := strings.Count(got.stderr, want); n != 1 {
		t.Fatalf("expected the notice once from the check and the header together, got %d in %q", n, got.stderr)
	}
}

func TestVersionPolicyCheckDisabled(t *testing.T) {
	env := newVersionPolicyEnv(t, &versionpolicy.Policy{Minimum: versionAbove(t)})
	env.config.Update.CheckEnabled = false

	got := env.run(t, "runtime", "status")

	if env.requests.Load() != 0 {
		t.Fatal("expected no policy request")
	}
	if got.code != 0 {
		t.Fatalf("expected the command to run, got exit %d stderr=%q", got.code, got.stderr)
	}
}

// TestVersionPolicySkippedCommands uses cobra's real help, completion and
// __complete commands, which the cliupdate unit test can only imitate.
func TestVersionPolicySkippedCommands(t *testing.T) {
	for _, args := range [][]string{
		{"version"},
		{"--version"},
		{"help"},
		{"help", "runtime"},
		{"completion", "bash"},
		{"__complete", "runtime", ""},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			env := newVersionPolicyEnv(t, &versionpolicy.Policy{Minimum: versionAbove(t)})

			got := env.run(t, args...)

			if env.requests.Load() != 0 {
				t.Fatal("expected no policy request")
			}
			if got.code != 0 {
				t.Fatalf("expected exit 0, got %d stderr=%q", got.code, got.stderr)
			}
		})
	}
}

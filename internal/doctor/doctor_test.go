package doctor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/installmethod"
)

// testOptions returns Options for a machine where the binary is on PATH,
// no proxy is set, and every host is baseURL.
func testOptions(t *testing.T, baseURL string) Options {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "bin", "tollbit")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return Options{
		Config: configuration.Config{
			App:         configuration.AppConfig{Name: "tollbit"},
			Runtime:     configuration.RuntimeConfig{StateDir: filepath.Join(dir, "state")},
			Credentials: configuration.CredentialsConfig{StorageDir: filepath.Join(dir, "credentials")},
			Auth:        configuration.AuthConfig{BaseURL: baseURL},
			Foundry:     configuration.FoundryConfig{BaseURL: baseURL + "/foundry"},
			Gateway:     configuration.GatewayConfig{BaseURL: baseURL},
		},
		Executable:    func() (string, error) { return exe, nil },
		LookPath:      func(string) (string, error) { return exe, nil },
		Getenv:        func(string) string { return "" },
		InstallMethod: func() installmethod.Method { return installmethod.MethodInstaller },
		Proxy:         func(*http.Request) (*url.URL, error) { return nil, nil },
		SystemProxy:   func() string { return "" },
		HostTimeout:   2 * time.Second,
	}
}

// trusting returns a transport that trusts srv's certificate.
func trusting(srv *httptest.Server) *http.Transport {
	return srv.Client().Transport.(*http.Transport)
}

func findCheck(t *testing.T, report Report, name string) Check {
	t.Helper()
	for _, c := range report.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no check %q in %+v", name, report.Checks)
	return Check{}
}

func hostOf(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func TestRunPassesWhenEverythingWorks(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	opts := testOptions(t, srv.URL)
	opts.Transport = trusting(srv)

	report := Run(context.Background(), opts)

	if !report.OK {
		t.Fatalf("expected OK report, got %+v", report.Checks)
	}
	host := findCheck(t, report, hostOf(t, srv.URL))
	if host.Status != StatusPass || !strings.Contains(host.Detail, "auth, foundry, gateway") {
		t.Fatalf("expected one passing check for the shared host, got %+v", host)
	}
	for _, name := range []string{"state", "credentials", "install"} {
		if c := findCheck(t, report, name); c.Status != StatusPass {
			t.Fatalf("expected %s to pass, got %+v", name, c)
		}
	}
}

func TestHostProbeUsesVersionPolicyForGatewayHost(t *testing.T) {
	var paths []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
	}))
	defer srv.Close()
	opts := testOptions(t, "https://auth.invalid")
	opts.Config.Foundry.BaseURL = srv.URL + "/foundry"
	opts.Config.Gateway.BaseURL = srv.URL
	opts.Transport = trusting(srv)

	Run(context.Background(), opts)

	if len(paths) != 1 || paths[0] != "/foundry/cli/v1/version-policy" {
		t.Fatalf("expected one version policy request, got %v", paths)
	}
}

func TestBlockedHostByProxy(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "blocked by policy", http.StatusForbidden)
	}))
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)
	proxyURL.User = url.UserPassword("alice", "hunter2")
	opts := testOptions(t, "https://gateway.example.invalid")
	opts.Proxy = func(*http.Request) (*url.URL, error) { return proxyURL, nil }

	report := Run(context.Background(), opts)

	if report.OK {
		t.Fatal("expected a failed report")
	}
	c := findCheck(t, report, "gateway.example.invalid")
	if c.Status != StatusFail || c.ErrorKind != KindProxyRejected || !strings.Contains(c.Detail, "CONNECT 403") {
		t.Fatalf("expected proxy_rejected with CONNECT 403, got %+v", c)
	}
	if strings.Contains(c.Detail, "hunter2") || strings.Contains(c.Detail, "alice") {
		t.Fatalf("proxy credentials leaked into %q", c.Detail)
	}
}

func TestBlockedHostByHTTP403(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Access denied by your organization", http.StatusForbidden)
	}))
	defer srv.Close()
	opts := testOptions(t, srv.URL)
	opts.Transport = trusting(srv)

	c := findCheck(t, Run(context.Background(), opts), hostOf(t, srv.URL))

	if c.Status != StatusFail || c.ErrorKind != "http_403" || c.Hint == "" {
		t.Fatalf("expected http_403 with a hint, got %+v", c)
	}
}

func TestTLSInterceptionNamesIssuer(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	// The default transport does not trust httptest's certificate, as a
	// machine does not trust an inspecting proxy's.
	opts := testOptions(t, srv.URL)

	c := findCheck(t, Run(context.Background(), opts), hostOf(t, srv.URL))

	if c.ErrorKind != KindTLSIntercepted || !strings.Contains(c.Detail, "Acme Co") {
		t.Fatalf("expected tls_intercepted naming the issuer, got %+v", c)
	}
}

func TestDNSFailure(t *testing.T) {
	opts := testOptions(t, "https://doctor-check.invalid")

	c := findCheck(t, Run(context.Background(), opts), "doctor-check.invalid")

	if c.ErrorKind != KindDNS {
		t.Fatalf("expected dns, got %+v", c)
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	opts := testOptions(t, srv.URL)
	opts.Transport = trusting(srv)
	opts.HostTimeout = 200 * time.Millisecond

	c := findCheck(t, Run(context.Background(), opts), hostOf(t, srv.URL))

	if c.ErrorKind != KindTimeout {
		t.Fatalf("expected timeout, got %+v", c)
	}
}

func TestUnreachableProxy(t *testing.T) {
	opts := testOptions(t, "https://gateway.example.invalid")
	opts.Proxy = func(*http.Request) (*url.URL, error) { return url.Parse("http://127.0.0.1:1") }

	c := findCheck(t, Run(context.Background(), opts), "gateway.example.invalid")

	if c.ErrorKind != KindProxyUnreachable {
		t.Fatalf("expected proxy_unreachable, got %+v", c)
	}
}

func TestReleaseProbeFollowsRedirectToAssetHost(t *testing.T) {
	assets := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer assets.Close()
	releases := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, assets.URL+"/asset", http.StatusFound)
	}))
	defer releases.Close()
	opts := testOptions(t, assets.URL)
	opts.Config.Update = configuration.UpdateConfig{CheckEnabled: true, ReleaseBaseURL: releases.URL + "/download"}
	// Both servers share httptest's certificate.
	opts.Transport = trusting(releases)

	c := findCheck(t, Run(context.Background(), opts), hostOf(t, releases.URL))

	if c.Status != StatusPass || !strings.Contains(c.Detail, "via "+hostOf(t, assets.URL)) {
		t.Fatalf("expected a pass naming the asset host, got %+v", c)
	}
}

func TestUnwritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits do not stop writes on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write to read-only directories")
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	readOnly := t.TempDir()
	if err := os.Chmod(readOnly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) })
	opts := testOptions(t, srv.URL)
	opts.Transport = trusting(srv)
	opts.Config.Runtime.StateDir = readOnly
	opts.Config.Credentials.StorageDir = filepath.Join(readOnly, "credentials")

	report := Run(context.Background(), opts)

	if report.OK {
		t.Fatal("expected a failed report")
	}
	state := findCheck(t, report, "state")
	if state.Status != StatusFail || !strings.Contains(state.Detail, "cannot write") || !strings.Contains(state.Hint, "TOLLBIT_RUNTIME_STATE_DIR") {
		t.Fatalf("expected state to fail to write, got %+v", state)
	}
	creds := findCheck(t, report, "credentials")
	if creds.Status != StatusFail || !strings.Contains(creds.Detail, "cannot create") {
		t.Fatalf("expected credentials to fail to create, got %+v", creds)
	}
}

func TestReadOnlyInstallDirOnlyWarns(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs directory permission bits that stop writes")
	}
	opts := testOptions(t, "https://unused.invalid")
	exe, _ := opts.Executable()
	if err := os.Chmod(filepath.Dir(exe), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(exe), 0o700) })

	c := findCheck(t, Run(context.Background(), opts), "install")

	if c.Status != StatusWarn {
		t.Fatalf("expected a warning, got %+v", c)
	}
}

func TestPathCheck(t *testing.T) {
	opts := testOptions(t, "https://unused.invalid").withDefaults()
	exe, _ := opts.Executable()
	other := filepath.Join(t.TempDir(), "tollbit")
	if err := os.WriteFile(other, []byte("other"), 0o755); err != nil {
		t.Fatal(err)
	}

	opts.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	if c := pathCheck(opts, exe, installmethod.MethodInstaller); c.Status != StatusFail || !strings.Contains(c.Hint, filepath.Dir(exe)) {
		t.Fatalf("expected fail naming the install dir, got %+v", c)
	}
	opts.LookPath = func(string) (string, error) { return other, nil }
	if c := pathCheck(opts, exe, installmethod.MethodInstaller); c.Status != StatusWarn {
		t.Fatalf("expected a warning for another binary on PATH, got %+v", c)
	}
	if c := pathCheck(opts, exe, installmethod.MethodNPM); c.Status != StatusPass {
		t.Fatalf("expected the npm wrapper to pass, got %+v", c)
	}
}

func TestProxyChecks(t *testing.T) {
	opts := testOptions(t, "https://unused.invalid")
	env := map[string]string{"HTTPS_PROXY": "http://bob:s3cret@proxy.corp:8080", "NO_PROXY": "localhost"}
	opts.Getenv = func(name string) string { return env[name] }
	opts.SystemProxy = func() string { return "proxy proxy.corp:8080" }

	checks := proxyChecks(opts)

	if len(checks) != 2 || strings.Contains(checks[0].Detail, "s3cret") || !strings.Contains(checks[0].Detail, "proxy.corp:8080") {
		t.Fatalf("expected redacted proxy env, got %+v", checks)
	}
	if checks[1].Status != StatusInfo {
		t.Fatalf("expected system proxy info when HTTPS_PROXY is set, got %+v", checks[1])
	}

	delete(env, "HTTPS_PROXY")
	if c := proxyChecks(opts)[1]; c.Status != StatusWarn {
		t.Fatalf("expected a warning when only the OS proxy is set, got %+v", c)
	}
}

func TestRedactProxy(t *testing.T) {
	for raw, want := range map[string]string{
		"http://proxy:8080":            "http://proxy:8080",
		"http://user:pass@proxy:8080":  "http://***@proxy:8080",
		"user:pass@proxy:8080":         "http://***@proxy:8080",
		"proxy:8080":                   "proxy:8080",
		"https://token@proxy.corp/x?y": "https://***@proxy.corp/x?y",
	} {
		if got := redactProxy(raw); got != want {
			t.Errorf("redactProxy(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestReleaseProbeStopsRedirectLoop(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+r.URL.Path, http.StatusFound)
	}))
	defer srv.Close()
	opts := testOptions(t, "https://unused.invalid")
	opts.Config.Update = configuration.UpdateConfig{CheckEnabled: true, ReleaseBaseURL: srv.URL + "/download"}
	opts.Transport = trusting(srv)

	c := findCheck(t, Run(context.Background(), opts), hostOf(t, srv.URL))

	if c.Status != StatusFail || c.ErrorKind == KindTimeout || !strings.Contains(c.Detail, "10 redirects") {
		t.Fatalf("expected a redirect failure, not a timeout, got %+v", c)
	}
}

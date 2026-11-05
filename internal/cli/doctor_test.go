package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/doctor"
	"github.com/tollbit/cli/internal/version"
)

func TestRunDoctorJSONReportsAuthWithoutPrintingTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	storageDir := t.TempDir()
	token := testAgentJWT(t)
	if err := os.WriteFile(filepath.Join(storageDir, "agent-token.jwt"), []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	refresh := `{"refresh_token":"refresh-secret-value"}`
	if err := os.WriteFile(filepath.Join(storageDir, "refresh-token.json"), []byte(refresh), 0o600); err != nil {
		t.Fatal(err)
	}
	config := testConfig()
	config.Runtime.StateDir = storageDir
	config.Credentials.StorageDir = storageDir
	config.Auth.BaseURL = srv.URL
	config.Foundry.BaseURL = srv.URL + "/foundry"
	config.Gateway.BaseURL = srv.URL
	config.Update.CheckEnabled = false

	var stdout, stderr bytes.Buffer
	executeTestCommandWithConfig(config, []string{"doctor", "--json"}, nil, &stdout, &stderr)

	output := stdout.String() + stderr.String()
	if strings.Contains(output, token) || strings.Contains(output, "refresh-secret-value") {
		t.Fatalf("doctor printed a credential: %s", output)
	}
	var report doctor.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("expected JSON report, got %q: %v", stdout.String(), err)
	}
	var auth, host *doctor.Check
	skills := map[string]string{}
	for i, c := range report.Checks {
		switch {
		case c.Group == doctor.GroupSkills:
			skills[c.Name] = c.Version
		case c.Group == doctor.GroupAuth:
			auth = &report.Checks[i]
		case c.Group == doctor.GroupNetwork && c.Host != "":
			host = &report.Checks[i]
		}
	}
	if auth == nil || !strings.Contains(auth.Detail, "token stored") || !strings.Contains(auth.Detail, "refresh token stored") {
		t.Fatalf("expected auth check to report stored tokens, got %+v", auth)
	}
	if host == nil || host.Status != doctor.StatusPass {
		t.Fatalf("expected the local host to pass, got %+v", host)
	}
	want := map[string]string{"tollbit-cli": version.Version, "tollbit-analytics": version.Version}
	if !reflect.DeepEqual(skills, want) {
		t.Fatalf("skills = %v, want %v", skills, want)
	}
}

func TestRunDoctorExitsNonZeroOnFailure(t *testing.T) {
	config := testConfig()
	config.Runtime.StateDir = t.TempDir()
	config.Credentials.StorageDir = config.Runtime.StateDir
	config.Auth.BaseURL = "https://doctor-check.invalid"
	config.Foundry.BaseURL = "https://doctor-check.invalid/foundry"
	config.Gateway.BaseURL = "https://doctor-check.invalid"
	config.Update.CheckEnabled = false

	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"doctor"}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit 1, got %d: %s %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "FAIL  doctor-check.invalid") || !strings.Contains(stdout.String(), "hint:") {
		t.Fatalf("expected a failing host line with a hint, got %q", stdout.String())
	}
}

// doctorTestConfig answers every host from a local server and puts a stand-in
// tollbit on PATH, so nothing but the skills under test can fail doctor.
func doctorTestConfig(t *testing.T) configuration.Config {
	t.Helper()
	bin := t.TempDir()
	for _, name := range []string{"tollbit", "tollbit.exe"} {
		if err := os.WriteFile(filepath.Join(bin, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(srv.Close)
	config := testConfig()
	config.Runtime.StateDir = t.TempDir()
	config.Credentials.StorageDir = config.Runtime.StateDir
	config.Auth.BaseURL = srv.URL
	config.Foundry.BaseURL = srv.URL + "/foundry"
	config.Gateway.BaseURL = srv.URL
	config.Update.CheckEnabled = false
	return config
}

func skillChecksByName(t *testing.T, output []byte) map[string]doctor.Check {
	t.Helper()
	var report doctor.Report
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("expected JSON report, got %q: %v", output, err)
	}
	checks := map[string]doctor.Check{}
	for _, c := range report.Checks {
		if c.Group == doctor.GroupSkills {
			checks[c.Name] = c
		}
	}
	return checks
}

func TestDoctorSkillsDirFindsWhatGuideInstalled(t *testing.T) {
	config := doctorTestConfig(t)
	dir := t.TempDir()
	for _, name := range []string{"cli", "analytics"} {
		var stdout, stderr bytes.Buffer
		if code := executeTestCommandWithConfig(config, []string{"guide", "--skill", name, "--install", dir}, nil, &stdout, &stderr); code != 0 {
			t.Fatalf("guide --install failed: %d %s", code, stderr.String())
		}
	}

	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"doctor", "--json", "--skills-dir", dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s %s", code, stdout.String(), stderr.String())
	}
	checks := skillChecksByName(t, stdout.Bytes())
	if len(checks) != 2 {
		t.Fatalf("got %v, want both skills", checks)
	}
	for _, name := range []string{"tollbit-cli", "tollbit-analytics"} {
		c := checks[name]
		if c.Status != doctor.StatusPass || c.InstalledVersion != version.Version || c.Path != filepath.Join(dir, name, "SKILL.md") {
			t.Fatalf("unexpected check for %s: %+v", name, c)
		}
	}
}

func TestDoctorSkillsDirWarnsWithoutFailing(t *testing.T) {
	config := doctorTestConfig(t)
	dir := t.TempDir()
	stale := filepath.Join(dir, "tollbit-cli")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "SKILL.md"), []byte("---\nname: tollbit-cli\nversion: 0.0.1\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"doctor", "--skills-dir", dir}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s %s", code, stdout.String(), stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"WARN  tollbit-cli",
		"version 0.0.1 installed at",
		"update with `tollbit guide --skill cli --install " + dir + "`",
		"WARN  tollbit-analytics",
		"install with `tollbit guide --skill analytics --install " + dir + "`",
		"All required checks passed",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output should contain %q, got %q", want, output)
		}
	}
	if strings.Contains(output, "Compare with the version: line") {
		t.Fatalf("the compare-by-hand note should not print with --skills-dir: %q", output)
	}
}

func TestDoctorWithoutSkillsDirListsBundledSkills(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(doctorTestConfig(t), []string{"doctor"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s %s", code, stdout.String(), stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Compare with the version: line of your installed SKILL.md.",
		"bundled with this CLI",
		"update with `tollbit guide --skill cli --install <skills-dir>`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output should contain %q, got %q", want, output)
		}
	}
}

func TestDoctorRejectsEmptySkillsDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(doctorTestConfig(t), []string{"doctor", "--skills-dir", ""}, nil, &stdout, &stderr)

	if code != 2 || !strings.Contains(stderr.String(), "--skills-dir") {
		t.Fatalf("expected a usage error, got %d: %s", code, stderr.String())
	}
}

func TestRunDoctorTextAndJSONGiveTheSameSandboxHint(t *testing.T) {
	config := testConfig()
	config.Runtime.StateDir = t.TempDir()
	config.Credentials.StorageDir = config.Runtime.StateDir
	config.Auth.BaseURL = "https://doctor-check.invalid"
	config.Foundry.BaseURL = "https://doctor-check.invalid/foundry"
	config.Gateway.BaseURL = "https://doctor-check.invalid"
	config.Update.CheckEnabled = false

	var jsonOut, textOut, stderr bytes.Buffer
	executeTestCommandWithConfig(config, []string{"doctor", "--json"}, nil, &jsonOut, &stderr)
	executeTestCommandWithConfig(config, []string{"doctor"}, nil, &textOut, &stderr)

	var report doctor.Report
	if err := json.Unmarshal(jsonOut.Bytes(), &report); err != nil {
		t.Fatalf("expected JSON report, got %q: %v", jsonOut.String(), err)
	}
	var hint string
	for _, c := range report.Checks {
		if c.Host == "doctor-check.invalid" {
			hint = c.Hint
		}
	}
	if !strings.HasPrefix(hint, "This may be a sandbox with no network.") {
		t.Fatalf("expected the JSON hint to lead with the sandbox, got %q", hint)
	}
	if !strings.Contains(textOut.String(), "hint: "+hint+"\n") {
		t.Fatalf("expected text output to carry the JSON hint %q, got %q", hint, textOut.String())
	}
}

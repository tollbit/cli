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

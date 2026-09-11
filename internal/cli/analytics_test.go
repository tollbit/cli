package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyticsCommandDisabled(t *testing.T) {
	config := testConfig()
	config.Analytics.Enabled = false

	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"analytics", "query", "SELECT 1"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected usage exit code, got %d (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), `unknown command "analytics"`) {
		t.Fatalf("expected disabled command to be unknown, got %q", stderr.String())
	}
}

func TestAnalyticsQueryUsesOBOAgentTokenAndWritesJSON(t *testing.T) {
	token := testAgentJWTWithOBO(t)
	storageDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(storageDir, "agent-token.jwt"), []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	analyticsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/analytics/agent/v1/query" {
			t.Fatalf("unexpected analytics request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Fatal("unexpected authorization header")
		}
		var request struct {
			SQL string `json:"sql"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.SQL != "SELECT * FROM logs" {
			t.Fatalf("unexpected SQL: %q", request.SQL)
		}
		_, _ = w.Write([]byte(`{"columns":[{"name":"requests","type":"INTEGER"},{"name":"optional","type":"STRING"}],"rows":[[42,null]]}`))
	}))
	defer analyticsSrv.Close()

	config := testConfig()
	config.Analytics.Enabled = true
	config.Analytics.BaseURL = analyticsSrv.URL
	config.Credentials.StorageDir = storageDir
	config.Runtime.StateDir = storageDir

	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"analytics", "query", "SELECT * FROM logs"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected success, got %d (stderr=%q)", code, stderr.String())
	}
	var output struct {
		Columns []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"columns"`
		Rows [][]any `json:"rows"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON output %q: %v", stdout.String(), err)
	}
	if len(output.Columns) != 2 || output.Columns[0].Name != "requests" {
		t.Fatalf("unexpected columns: %#v", output.Columns)
	}
	if len(output.Rows) != 1 || output.Rows[0][0] != float64(42) || output.Rows[0][1] != nil {
		t.Fatalf("unexpected rows: %#v", output.Rows)
	}
}

func TestAnalyticsQueryRequiresOneSQLArgument(t *testing.T) {
	config := testConfig()
	config.Analytics.Enabled = true
	config.Analytics.BaseURL = "https://analytics.example"

	for _, args := range [][]string{
		{"analytics", "query"},
		{"analytics", "query", "SELECT", "1"},
	} {
		var stdout, stderr bytes.Buffer
		code := executeTestCommandWithConfig(config, args, nil, &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "analytics query requires <SQL>") {
			t.Fatalf("expected SQL usage error for %#v, got code=%d stderr=%q", args, code, stderr.String())
		}
	}
}

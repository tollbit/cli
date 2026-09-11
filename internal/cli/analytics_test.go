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

func TestAnalyticsSchemaUsesOBOAgentTokenAndWritesJSON(t *testing.T) {
	token := testAgentJWTWithOBO(t)
	storageDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(storageDir, "agent-token.jwt"), []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	analyticsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/analytics/agent/v1/query/schema" {
			t.Fatalf("unexpected analytics request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Fatal("unexpected authorization header")
		}
		_, _ = w.Write([]byte(`[{"name":"agent_logs_by_page","columns":[{"name":"host","type":"STRING"}]}]`))
	}))
	defer analyticsSrv.Close()

	config := testConfig()
	config.Analytics.Enabled = true
	config.Analytics.BaseURL = analyticsSrv.URL
	config.Credentials.StorageDir = storageDir
	config.Runtime.StateDir = storageDir

	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"analytics", "schema"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected success, got %d (stderr=%q)", code, stderr.String())
	}
	var output []struct {
		Name    string `json:"name"`
		Columns []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"columns"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON output %q: %v", stdout.String(), err)
	}
	if len(output) != 1 || output[0].Name != "agent_logs_by_page" {
		t.Fatalf("unexpected tables: %#v", output)
	}
	if len(output[0].Columns) != 1 || output[0].Columns[0].Name != "host" {
		t.Fatalf("unexpected columns: %#v", output[0].Columns)
	}
}

func TestAnalyticsQueryRequiresOneSQLArgument(t *testing.T) {
	config := testConfig()
	config.Analytics.Enabled = true
	config.Analytics.BaseURL = "https://analytics.example"

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"analytics", "query"}, "analytics query requires <SQL>"},
		{[]string{"analytics", "query", "SELECT", "1"}, "analytics query accepts a single <SQL> argument"},
		{[]string{"analytics", "query", "   "}, "analytics query SQL must not be empty"},
	} {
		var stdout, stderr bytes.Buffer
		code := executeTestCommandWithConfig(config, tc.args, nil, &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), tc.want) {
			t.Fatalf("expected usage error %q for %#v, got code=%d stderr=%q", tc.want, tc.args, code, stderr.String())
		}
	}
}

func TestAnalyticsCommandsRejectUserAgentFlag(t *testing.T) {
	config := testConfig()
	config.Analytics.Enabled = true
	config.Analytics.BaseURL = "https://analytics.example"

	for _, args := range [][]string{
		{"analytics", "query", "--user-agent", "Agent", "SELECT 1"},
		{"analytics", "schema", "--user-agent", "Agent"},
	} {
		var stdout, stderr bytes.Buffer
		code := executeTestCommandWithConfig(config, args, nil, &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "unknown flag: --user-agent") {
			t.Fatalf("expected unknown flag error for %#v, got code=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func TestAnalyticsHelpDocumentsQueryContract(t *testing.T) {
	config := testConfig()
	config.Analytics.Enabled = true

	for _, tc := range []struct {
		args []string
		want []string
	}{
		{[]string{"analytics", "--help"}, []string{"analytics schema", "analytics query"}},
		{[]string{"analytics", "query", "--help"}, []string{"BigQuery Standard SQL", "single SELECT", "timestamp", "user_agent_aggregate", "Examples:"}},
		{[]string{"analytics", "schema", "--help"}, []string{"JSON array of tables", "analytics query"}},
	} {
		var stdout, stderr bytes.Buffer
		code := executeTestCommandWithConfig(config, tc.args, nil, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected help to succeed for %#v, got code=%d stderr=%q", tc.args, code, stderr.String())
		}
		for _, want := range tc.want {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("expected help for %#v to mention %q, got:\n%s", tc.args, want, stdout.String())
			}
		}
		if strings.Contains(stdout.String(), "FROM logs") {
			t.Fatalf("help for %#v still references the nonexistent logs table", tc.args)
		}
	}
}

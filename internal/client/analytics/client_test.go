package analytics

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tollbit/cli/internal/errorsx/problemjson"
	"github.com/tollbit/cli/internal/tokens/agent"
)

func TestQuery(t *testing.T) {
	token := validAgentToken(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/prefix/analytics/agent/v1/query" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected content headers: %#v", r.Header)
		}
		if r.Header.Get("Authorization") != "Bearer "+token.RawToken {
			t.Fatal("unexpected authorization header")
		}
		var request QueryRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.SQL != "SELECT * FROM logs" {
			t.Fatalf("unexpected SQL: %q", request.SQL)
		}
		_, _ = w.Write([]byte(`{"columns":[{"name":"requests","type":"INTEGER"},{"name":"optional","type":"STRING"}],"rows":[[42,null]],"meta":{"row_count":1,"truncated":true,"bytes_scanned":512,"duration_ms":7}}`))
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: " " + srv.URL + "/prefix "})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Query(context.Background(), QueryRequest{SQL: "SELECT * FROM logs"}, token)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Columns) != 2 || response.Columns[0].Name != "requests" {
		t.Fatalf("unexpected columns: %#v", response.Columns)
	}
	if len(response.Rows) != 1 || response.Rows[0][0] != float64(42) || response.Rows[0][1] != nil {
		t.Fatalf("unexpected rows: %#v", response.Rows)
	}
	if response.Meta == nil || !response.Meta.Truncated || response.Meta.RowCount != 1 || response.Meta.BytesScanned != 512 || response.Meta.DurationMs != 7 {
		t.Fatalf("unexpected meta: %#v", response.Meta)
	}
}

func TestQueryWithoutMeta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"columns":[{"name":"n","type":"INT64"}],"rows":[[1]]}`))
	}))
	defer srv.Close()
	client, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Query(context.Background(), QueryRequest{SQL: "SELECT 1"}, validAgentToken(t))
	if err != nil {
		t.Fatal(err)
	}
	if response.Meta != nil {
		t.Fatalf("expected no meta from an older server, got %#v", response.Meta)
	}
}

func TestSchema(t *testing.T) {
	token := validAgentToken(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/prefix/analytics/agent/v1/query/schema" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Fatalf("unexpected accept header: %q", r.Header.Get("Accept"))
		}
		if r.Header.Get("Authorization") != "Bearer "+token.RawToken {
			t.Fatal("unexpected authorization header")
		}
		_, _ = w.Write([]byte(`{
			"dialect": "standard_sql",
			"tables": [{"name": "agent_logs_by_page", "description": "Daily counts.", "clustering": ["host", "user_agent", "path"],
			            "columns": [{"name": "host", "type": "STRING", "description": "Site hostname."},
			                        {"name": "type", "type": "STRING", "values": ["REQUEST", "ROBOT"]}]}],
			"limits": {"max_rows": {"value": 10000, "unit": "rows", "description": "Row cap."},
			           "something_new": {"value": 3, "unit": "widgets"}}
		}`))
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: " " + srv.URL + "/prefix "})
	if err != nil {
		t.Fatal(err)
	}
	schema, err := client.Schema(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if schema.Dialect != "standard_sql" {
		t.Fatalf("unexpected dialect: %q", schema.Dialect)
	}
	if len(schema.Tables) != 1 || schema.Tables[0].Name != "agent_logs_by_page" || schema.Tables[0].Description != "Daily counts." {
		t.Fatalf("unexpected tables: %#v", schema.Tables)
	}
	if got := schema.Tables[0].Clustering; len(got) != 3 || got[1] != "user_agent" {
		t.Fatalf("unexpected clustering: %#v", got)
	}
	cols := schema.Tables[0].Columns
	if len(cols) != 2 || cols[0].Description != "Site hostname." || len(cols[1].Values) != 2 {
		t.Fatalf("unexpected columns: %#v", cols)
	}
	if len(schema.Limits) != 2 || schema.Limits["max_rows"].Unit != "rows" || schema.Limits["something_new"].Value.String() != "3" {
		t.Fatalf("unexpected limits: %#v", schema.Limits)
	}
}

func TestSchemaAcceptsLegacyArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(` [{"name":"agent_logs_by_page","columns":[{"name":"host","type":"STRING"}]}]`))
	}))
	defer srv.Close()
	client, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	schema, err := client.Schema(context.Background(), validAgentToken(t))
	if err != nil {
		t.Fatal(err)
	}
	if schema.Dialect != "" || schema.Limits != nil {
		t.Fatalf("legacy array must not invent dialect or limits: %#v", schema)
	}
	if len(schema.Tables) != 1 || schema.Tables[0].Name != "agent_logs_by_page" || schema.Tables[0].Columns[0].Name != "host" {
		t.Fatalf("unexpected tables: %#v", schema.Tables)
	}
}

func TestQueryParsesProblemJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "request-123")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"title":"Forbidden","status":403,"detail":"organization access required"}`))
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(context.Background(), QueryRequest{SQL: "SELECT 1"}, validAgentToken(t))
	var problem problemjson.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("expected ProblemJSON error, got %v", err)
	}
	if problem.RequestID == nil || *problem.RequestID != "request-123" {
		t.Fatalf("unexpected request ID: %#v", problem.RequestID)
	}
}

func TestQueryRejectsMissingToken(t *testing.T) {
	client, err := NewClient(Config{BaseURL: "https://analytics.example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Query(context.Background(), QueryRequest{SQL: "SELECT 1"}, agent.Token{}); err == nil {
		t.Fatal("expected missing token error")
	}
}

func TestNewClientRequiresBaseURL(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Fatal("expected missing base URL error")
	}
}

func validAgentToken(t *testing.T) agent.Token {
	t.Helper()
	claims := struct {
		jwt.RegisteredClaims
		TBT string `json:"tbt"`
	}{
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		TBT:              "agent-token",
	}
	header, err := json.Marshal(map[string]any{"alg": "none"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	signature := base64.RawURLEncoding.EncodeToString([]byte("signature"))
	return agent.Token{RawToken: base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + "." + signature}
}

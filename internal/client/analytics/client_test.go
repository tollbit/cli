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
		_ = json.NewEncoder(w).Encode(QueryResponse{
			Columns: []QueryColumn{{Name: "requests", Type: "INTEGER"}, {Name: "optional", Type: "STRING"}},
			Rows:    [][]any{{42, nil}},
		})
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

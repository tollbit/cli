package tollbit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tollbit/tollbit-cli/internal/errorsx/problemjson"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
)

func TestConfigNormalize(t *testing.T) {
	cfg := Config{BaseURL: " https://gateway.example.com "}
	cfg.Normalize()
	if cfg.BaseURL != "https://gateway.example.com" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
}

func TestNewClientRequiresBaseURL(t *testing.T) {
	_, err := NewClient(Config{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSearch(t *testing.T) {
	token := validAgentToken(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/agents/v1/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "climate policy" {
			t.Fatalf("unexpected q: %q", r.URL.Query().Get("q"))
		}
		if r.URL.Query().Get("size") != "5" {
			t.Fatalf("unexpected size: %q", r.URL.Query().Get("size"))
		}
		if r.URL.Query().Get("next-token") != "page-2" {
			t.Fatalf("unexpected next-token: %q", r.URL.Query().Get("next-token"))
		}
		if r.URL.Query().Get("properties") != "example.com,other.com" {
			t.Fatalf("unexpected properties: %q", r.URL.Query().Get("properties"))
		}
		if r.URL.Query().Get("allowedOnly") != "true" {
			t.Fatalf("unexpected allowedOnly: %q", r.URL.Query().Get("allowedOnly"))
		}
		if r.Header.Get("Authorization") != "Bearer "+token.RawToken {
			t.Fatalf("unexpected authorization: %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(PagedSearchResultResponse{
			NextToken: "page-3",
			Items: []SearchResult{{
				Title:         "Article",
				URL:           "https://example.com/article",
				PublishedDate: "2024-01-01",
				Publisher:     Publisher{Domain: "example.com", Name: "Example"},
				Availability:  Availability{Discoverable: true, ReadyToLicense: false},
			}},
		})
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Search(context.Background(), SearchParams{
		Query:          "climate policy",
		Size:           5,
		NextToken:      "page-2",
		Properties:     "example.com,other.com",
		AllowedOnly:    true,
		AllowedOnlySet: true,
	}, token)
	if err != nil {
		t.Fatal(err)
	}
	if resp.NextToken != "page-3" || len(resp.Items) != 1 {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if resp.Items[0].Title != "Article" {
		t.Fatalf("unexpected item title: %q", resp.Items[0].Title)
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	c, err := NewClient(Config{BaseURL: "https://gateway.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Search(context.Background(), SearchParams{}, validAgentToken(t))
	if err == nil || !strings.Contains(err.Error(), "search query is required") {
		t.Fatalf("expected query required error, got %v", err)
	}
}

func TestSearchReturnsProblemJSONError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(problemjson.Problem{
			Type:   "about:blank",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: strPtr("invalid query"),
		})
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Search(context.Background(), SearchParams{Query: "test"}, validAgentToken(t))
	if err == nil {
		t.Fatal("expected error")
	}
	var problem problemjson.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("expected problemjson error, got %T: %v", err, err)
	}
	if problem.Status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", problem.Status)
	}
}

func validAgentToken(t *testing.T) agent.Token {
	t.Helper()
	now := time.Now()
	claims := jwt.MapClaims{
		"tbt": "agent-token",
		"exp": now.Add(time.Hour).Unix(),
		"nbf": now.Add(-time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	return agent.Token{RawToken: signed}
}

func strPtr(s string) *string {
	return &s
}

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

func TestBatchGetRates(t *testing.T) {
	token := validAgentToken(t)
	urls := []string{"https://example.com/a", "https://example.com/b"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/agents/v1/rates/batch" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+token.RawToken {
			t.Fatalf("unexpected authorization: %q", r.Header.Get("Authorization"))
		}
		var req BatchGetRateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if len(req.URLs) != 2 || req.URLs[0] != urls[0] || req.URLs[1] != urls[1] {
			t.Fatalf("unexpected request body: %#v", req)
		}
		_ = json.NewEncoder(w).Encode([]BatchRateResponseV2{{
			URL: urls[0],
			Rates: []BatchDeveloperRateResponse{{
				Price:   RatePriceResponse{PriceMicros: 50000, Currency: "USD"},
				License: BatchRateLicenseResponse{LicenseType: "standard"},
			}},
		}})
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.BatchGetRates(context.Background(), urls, token, "MyAgent-User")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp) != 1 || resp[0].URL != urls[0] {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if len(resp[0].Rates) != 1 || resp[0].Rates[0].License.LicenseType != "standard" {
		t.Fatalf("unexpected rates: %#v", resp[0].Rates)
	}
}

func TestBatchGetRatesRequiresURLs(t *testing.T) {
	c, err := NewClient(Config{BaseURL: "https://gateway.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.BatchGetRates(context.Background(), nil, validAgentToken(t), "")
	if err == nil || !strings.Contains(err.Error(), "at least one URL is required") {
		t.Fatalf("expected URL required error, got %v", err)
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

func TestCreateContentAccessToken(t *testing.T) {
	token := validAgentToken(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/agents/v1/tokens/content" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+token.RawToken {
			t.Fatalf("unexpected authorization: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("User-Agent") != "MyAgent-User" {
			t.Fatalf("unexpected user agent: %q", r.Header.Get("User-Agent"))
		}
		var req CreateContentAccessTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.URL != "https://example.com/article" || req.UserAgent != "MyAgent-User" {
			t.Fatalf("unexpected request body: %#v", req)
		}
		_ = json.NewEncoder(w).Encode(CreateContentAccessTokenResponse{Token: "content-jwt"})
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.CreateContentAccessToken(context.Background(), CreateContentAccessTokenRequest{
		URL:            "https://example.com/article",
		UserAgent:      "MyAgent-User",
		MaxPriceMicros: 50000,
		Currency:       "USD",
		LicenseType:    "ON_DEMAND_LICENSE",
		LicenseCuid:    "lic_1",
		Format:         "markdown",
	}, token, "MyAgent-User")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Token != "content-jwt" {
		t.Fatalf("unexpected token: %q", resp.Token)
	}
}

func TestGetContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/dev/v2/content/example.com/article" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("TollbitToken") != "content-jwt" {
			t.Fatalf("unexpected tollbit token: %q", r.Header.Get("TollbitToken"))
		}
		if r.Header.Get("Tollbit-User-Agent") != "MyAgent-User" {
			t.Fatalf("unexpected tollbit user agent: %q", r.Header.Get("Tollbit-User-Agent"))
		}
		if r.Header.Get("User-Agent") != "MyAgent-User/1.0" {
			t.Fatalf("unexpected user agent: %q", r.Header.Get("User-Agent"))
		}
		if r.Header.Get("Tollbit-Accept-Content") != "text/markdown" {
			t.Fatalf("unexpected accept content: %q", r.Header.Get("Tollbit-Accept-Content"))
		}
		_ = json.NewEncoder(w).Encode(GetContentResponse{
			Content: PageContent{Body: "article body"},
		})
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.GetContent(context.Background(), "https://example.com/article", "content-jwt", "MyAgent-User")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content.Body != "article body" {
		t.Fatalf("unexpected body: %q", resp.Content.Body)
	}
}

func TestGetContentStripsTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dev/v2/content/example.com/article" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(GetContentResponse{Content: PageContent{Body: "ok"}})
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetContent(context.Background(), "https://example.com/article/", "content-jwt", "MyAgent-User")
	if err != nil {
		t.Fatal(err)
	}
}

func TestListUserAgents(t *testing.T) {
	token := validAgentToken(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/agents/v1/user-agents" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]UserAgentResponse{{
			Cuid: "ua_1", OrgCuid: "org_1", UserAgent: "MyAgent-User",
		}})
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.ListUserAgents(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp) != 1 || resp[0].UserAgent != "MyAgent-User" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestContentRequestUserAgent(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"MyAgent-User", "MyAgent-User/1.0"},
		{"MyAgent-User/1.0", "MyAgent-User/1.0"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := contentRequestUserAgent(tc.in); got != tc.want {
			t.Fatalf("contentRequestUserAgent(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

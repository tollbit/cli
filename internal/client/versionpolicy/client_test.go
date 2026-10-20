package versionpolicy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tollbit/cli/internal/version"
)

func TestGetReadsPolicyWithClientHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/foundry/cli/v1/version-policy" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Tollbit-Client"); got != version.ClientHeader() {
			t.Errorf("X-Tollbit-Client = %q, want %q", got, version.ClientHeader())
		}
		if got := r.Header.Get("User-Agent"); got != version.HTTPUserAgent() {
			t.Errorf("User-Agent = %q, want %q", got, version.HTTPUserAgent())
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("expected no Authorization header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"minimum":"0.3.0","latest":"0.5.1"}`))
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL + "/foundry/"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != (Policy{Minimum: "0.3.0", Latest: "0.5.1"}) {
		t.Fatalf("unexpected policy %#v", got)
	}
}

func TestGetReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

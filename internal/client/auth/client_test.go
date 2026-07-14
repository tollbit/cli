package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
	"github.com/tollbit/tollbit-cli/internal/version"
)

func TestClientCreatesAgentToken(t *testing.T) {
	ttl := int32(3600)
	var sawRequest bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI != "/agent/v1/tokens/identity" {
			t.Fatalf("unexpected request path: %s", r.RequestURI)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Fatalf("expected Accept application/json, got %q", r.Header.Get("Accept"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("User-Agent") != version.HTTPUserAgent() {
			t.Fatalf("expected User-Agent %q, got %q", version.HTTPUserAgent(), r.Header.Get("User-Agent"))
		}

		var body AgentTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.AgentIdentifier != "agent-test" {
			t.Fatalf("expected agent identifier, got %q", body.AgentIdentifier)
		}
		if body.TTLSeconds == nil || *body.TTLSeconds != ttl {
			t.Fatalf("expected ttl %d, got %#v", ttl, body.TTLSeconds)
		}
		if body.UA == nil || *body.UA != "agent-test/0.1" {
			t.Fatalf("expected ua from credentials, got %#v", body.UA)
		}
		if body.WBA == nil || body.WBA.Ver != 1 || body.WBA.Dir != "https://example.com/.well-known/web-bot-auth" || !body.WBA.Req {
			t.Fatalf("unexpected wba: %#v", body.WBA)
		}

		sawRequest = true
		_ = json.NewEncoder(w).Encode(AgentTokenResponse{Token: "agent-token"})
	}))
	defer srv.Close()

	c, err := New(ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	token, err := c.CreateAgentToken(context.Background(), AgentIdentity{
		Name:      "agent-test",
		UserAgent: "agent-test/0.1",
		WBA: &WebBotAuth{
			Ver: 1,
			Dir: "https://example.com/.well-known/web-bot-auth",
			Req: true,
		},
	}, AgentTokenOptions{
		TTLSeconds: &ttl,
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.RawToken != "agent-token" {
		t.Fatalf("expected token, got %q", token.RawToken)
	}
	if !sawRequest {
		t.Fatal("expected request to auth service")
	}
}

func TestClientOmitsOptionalUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if _, ok := body["ua"]; ok {
			t.Fatalf("expected ua to be omitted, got %#v", body["ua"])
		}
		_ = json.NewEncoder(w).Encode(AgentTokenResponse{Token: "agent-token"})
	}))
	defer srv.Close()

	c, err := New(ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateAgentToken(context.Background(), AgentIdentity{Name: "agent-test"}, AgentTokenOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestClientRequiresAgentName(t *testing.T) {
	c, err := New(ClientConfig{BaseURL: "https://auth.example"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.CreateAgentToken(context.Background(), AgentIdentity{}, AgentTokenOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "agent name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientStartAgentConsentRedirect(t *testing.T) {
	var sawRequest bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.RequestURI() != "/agent/v1/consent/redirect/start" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
		}
		if r.Header.Get("Authorization") != "Bearer unlinked-token" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		var body ConsentRedirectStartRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.RedirectURI != "http://127.0.0.1:1234/callback" || body.State != "state" || body.CodeChallenge != "challenge" || body.CodeChallengeMethod != "S256" {
			t.Fatalf("unexpected body: %#v", body)
		}
		sawRequest = true
		_ = json.NewEncoder(w).Encode(ConsentRedirectStartResponse{
			ChallengeID: "ach_123",
			ConsentURL:  "https://hack.tollbit.com/oauth/consent-new?consent_challenge=ach_123",
			ExpiresAt:   "2026-06-02T12:00:00Z",
		})
	}))
	defer srv.Close()

	c, err := New(ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.StartAgentConsentRedirect(context.Background(), agent.Token{RawToken: "unlinked-token"}, ConsentRedirectStartRequest{
		RedirectURI:   "http://127.0.0.1:1234/callback",
		State:         "state",
		CodeChallenge: "challenge",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawRequest || resp.ChallengeID != "ach_123" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestClientRedeemAgentConsentRedirect(t *testing.T) {
	var sawRequest bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.RequestURI() != "/agent/v1/tokens/identity" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
		}
		if r.Header.Get("Authorization") != "Bearer unlinked-token" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		var body ConsentRedirectTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.AgentIdentifier != "agent-test" || body.Code != "code" || body.CodeVerifier != "verifier" || body.RedirectURI != "http://127.0.0.1:1234/callback" {
			t.Fatalf("unexpected body: %#v", body)
		}
		sawRequest = true
		_ = json.NewEncoder(w).Encode(AgentTokenResponse{Token: "linked-token"})
	}))
	defer srv.Close()

	c, err := New(ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.RedeemAgentConsentRedirect(context.Background(), agent.Token{RawToken: "unlinked-token"}, ConsentRedirectTokenRequest{
		AgentIdentifier: "agent-test",
		Code:            "code",
		CodeVerifier:    "verifier",
		RedirectURI:     "http://127.0.0.1:1234/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawRequest || resp.Token != "linked-token" {
		t.Fatalf("unexpected token response: %#v", resp)
	}
}

func TestClientRefreshAgentToken(t *testing.T) {
	var sawRequest bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.RequestURI() != "/agent/v1/tokens/identity" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
		}
		if r.Header.Get("Authorization") != "Bearer base-token" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		var body identityTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.GrantType != "refresh_token" || body.AgentIdentifier != "agent-test" || body.RefreshToken != "agrt_old" {
			t.Fatalf("unexpected body: %#v", body)
		}
		sawRequest = true
		newRefreshToken := "agrt_new"
		refreshExpiresAt := "2026-07-08T12:00:00Z"
		_ = json.NewEncoder(w).Encode(AgentTokenResponse{Token: "new-obo-token", RefreshToken: &newRefreshToken, RefreshTokenExpiresAt: &refreshExpiresAt})
	}))
	defer srv.Close()

	c, err := New(ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.RefreshAgentToken(context.Background(), agent.Token{RawToken: "base-token"}, RefreshTokenGrantRequest{
		AgentIdentifier: "agent-test",
		RefreshToken:    "agrt_old",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawRequest || resp.Token != "new-obo-token" || resp.RefreshToken == nil || *resp.RefreshToken != "agrt_new" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestClientRevokesRefreshToken(t *testing.T) {
	var sawRequest bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.RequestURI() != "/agent/v1/tokens/refresh/revoke" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
		}
		var body RevokeRefreshTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.RefreshToken != "agrt_old" {
			t.Fatalf("unexpected body: %#v", body)
		}
		sawRequest = true
		_ = json.NewEncoder(w).Encode(RevokeRefreshTokenResponse{Revoked: true})
	}))
	defer srv.Close()

	c, err := New(ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.RevokeRefreshToken(context.Background(), " agrt_old ")
	if err != nil {
		t.Fatal(err)
	}
	if !sawRequest || !resp.Revoked {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestNewRequiresBaseURL(t *testing.T) {
	_, err := New(ClientConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "auth base URL is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientSurfacesProblemJSON(t *testing.T) {
	detail := "invalid token request"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"detail": detail,
			"status": http.StatusBadRequest,
			"title":  "Bad Request",
			"type":   "about:blank",
		})
	}))
	defer srv.Close()

	c, err := New(ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.CreateAgentToken(context.Background(), AgentIdentity{Name: "agent-test"}, AgentTokenOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), detail) {
		t.Fatalf("expected error detail %q, got %v", detail, err)
	}
}

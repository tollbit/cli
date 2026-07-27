package agentconfirmsicons

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/client/auth"
	"github.com/tollbit/cli/internal/cliruntime"
	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/tokens/agent"
)

func TestConsentStrategyStartsDetachedAuthorization(t *testing.T) {
	baseToken := testAgentJWT(t)
	var sawStart bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/agent/v1/consent/agent-confirms-icons/start" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+baseToken {
			t.Fatalf("unexpected authorization: %q", r.Header.Get("Authorization"))
		}
		var body auth.ConsentAgentConfirmsIconsStartRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.CodeChallenge == "" || body.CodeChallengeMethod != "S256" || body.Scope != "offline_access" {
			t.Fatalf("unexpected body: %#v", body)
		}
		sawStart = true
		_ = json.NewEncoder(w).Encode(auth.ConsentAgentConfirmsIconsStartResponse{
			ChallengeID: "ach_123",
			ConsentURL:  "https://auth.example.test/oauth/consent/agent-confirms-icons?consent_challenge=ach_123",
			ExpiresAt:   "2026-07-20T12:00:00Z",
			IconNames:   []string{"ANCHOR", "FOX", "STAR"},
		})
	}))
	defer srv.Close()
	client, err := auth.New(auth.ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	strategy, err := NewConsentStrategy(ConsentStrategyConfig{
		AuthClient:       client,
		Runtime:          newTestRuntime(t),
		UseRefreshTokens: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	inv := &testInvocation{}

	_, err = strategy.AuthorizeOBO(inv, auth.AgentIdentity{Name: "agent-test"}, agent.Token{RawToken: baseToken})

	var pending agentauth.AuthorizationPendingError
	if !errors.As(err, &pending) {
		t.Fatalf("expected pending authorization error, got %v", err)
	}
	if !sawStart || pending.Pending.ChallengeID != "ach_123" || pending.Pending.CodeVerifier == "" || len(pending.Pending.IconNames) != 3 {
		t.Fatalf("unexpected pending state: %#v", pending.Pending)
	}
	stdout := inv.stdout.String()
	for _, want := range []string{"Authorize agent: agent-test", "Relay ONLY this URL", "VALID ICON NAMES", "do NOT send this list to the user", "ANCHOR FOX STAR", "tollbit auth complete <first> <second> <third>", "Order matters"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
		}
	}
	if strategy.Guidance().CompleteArgsLabel == "" {
		t.Fatal("expected complete args guidance label")
	}
}

func TestConsentStrategyCompleteDetachedRedeems(t *testing.T) {
	baseToken := testAgentJWT(t)
	var sawRedeem bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/agent/v1/tokens/identity" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		icons := body["icon_names"].([]any)
		if body["grant_type"] != "consent:agent_confirms_icons" || body["challenge_id"] != "ach_123" || body["code_verifier"] != "verifier" || icons[0] != "FOX" || icons[1] != "ANCHOR" || icons[2] != "STAR" {
			t.Fatalf("unexpected body: %#v", body)
		}
		sawRedeem = true
		_ = json.NewEncoder(w).Encode(auth.AgentTokenResponse{Token: "linked-token"})
	}))
	defer srv.Close()
	strategy := newTestStrategy(t, srv.URL)

	resp, err := strategy.CompleteDetached(&testInvocation{}, testPending(baseToken), agentauth.CompleteDetachedInput{IconNames: []string{"fox,", "Anchor", " STAR "}})
	if err != nil {
		t.Fatal(err)
	}
	if !sawRedeem || resp.Token != "linked-token" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestCompleteDetachedRejectsBadCountWithoutServerCall(t *testing.T) {
	baseToken := testAgentJWT(t)
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer srv.Close()
	strategy := newTestStrategy(t, srv.URL)

	for _, input := range [][]string{nil, []string{"fox"}, []string{"fox", "anchor"}, []string{"fox", "anchor", "star", "owl"}} {
		_, err := strategy.CompleteDetached(&testInvocation{}, testPending(baseToken), agentauth.CompleteDetachedInput{IconNames: input})
		if err == nil || !strings.Contains(err.Error(), "expected exactly 3 icon names") {
			t.Fatalf("expected count error, got %v", err)
		}
	}
	if requests != 0 {
		t.Fatalf("expected no server requests, got %d", requests)
	}
}

func TestNewConsentStrategyValidatesConfig(t *testing.T) {
	client, err := auth.New(auth.ClientConfig{BaseURL: "https://auth.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		config  ConsentStrategyConfig
		wantErr string
	}{
		{name: "missing auth client", config: ConsentStrategyConfig{Runtime: newTestRuntime(t)}, wantErr: "auth client is required"},
		{name: "missing runtime", config: ConsentStrategyConfig{AuthClient: client}, wantErr: "runtime is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConsentStrategy(tt.config)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func newTestStrategy(t *testing.T, baseURL string) *ConsentStrategy {
	t.Helper()
	client, err := auth.New(auth.ClientConfig{BaseURL: baseURL})
	if err != nil {
		t.Fatal(err)
	}
	strategy, err := NewConsentStrategy(ConsentStrategyConfig{AuthClient: client, Runtime: newTestRuntime(t)})
	if err != nil {
		t.Fatal(err)
	}
	return strategy
}

func testPending(baseToken string) agentauth.PendingConsent {
	return agentauth.PendingConsent{
		Method:        agentauth.ConsentMethodAgentConfirmsIcons,
		ChallengeID:   "ach_123",
		AgentIdentity: auth.AgentIdentity{Name: "agent-test"},
		BaseToken:     baseToken,
		CodeVerifier:  "verifier",
		IconNames:     []string{"ANCHOR", "FOX", "STAR"},
	}
}

type testInvocation struct {
	stdout bytes.Buffer
	stderr bytes.Buffer
}

func (i *testInvocation) Context() context.Context { return context.Background() }
func (i *testInvocation) OutOrStdout() io.Writer   { return &i.stdout }
func (i *testInvocation) ErrOrStderr() io.Writer   { return &i.stderr }

func testAgentJWT(t *testing.T) string {
	t.Helper()
	claims := agent.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "agent-test", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		TBT:              "agent-token",
	}
	header := map[string]any{"alg": "none"}
	return encodeJSONSegment(t, header) + "." + encodeJSONSegment(t, claims) + "." + base64.RawURLEncoding.EncodeToString([]byte("signature"))
}

func encodeJSONSegment(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func newTestRuntime(t *testing.T) *cliruntime.Runtime {
	t.Helper()
	rt, err := cliruntime.New(cliruntime.Config{
		ConfiguredEndUserProximity: configuration.RuntimeEndUserProximityRemote,
		StateDir:                   t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

package codeauthorization

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
	"github.com/tollbit/cli/internal/errorsx/problemjson"
	"github.com/tollbit/cli/internal/tokens/agent"
)

func TestConsentStrategyStartsDetachedAuthorization(t *testing.T) {
	baseToken := testAgentJWT(t)
	var sawStart bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/agent/v1/consent/code-authorization/start" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+baseToken {
			t.Fatalf("unexpected authorization: %q", r.Header.Get("Authorization"))
		}
		var body auth.ConsentCodeAuthorizationStartRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.CodeChallenge == "" || body.CodeChallengeMethod != "S256" || body.Scope != "offline_access" {
			t.Fatalf("unexpected body: %#v", body)
		}
		sawStart = true
		_ = json.NewEncoder(w).Encode(auth.ConsentCodeAuthorizationStartResponse{
			ChallengeID: "ach_123",
			ConsentURL:  "https://tollbit.example.test/build/oauth/consent/code-authorization?consent_challenge=ach_123",
			ExpiresAt:   "2026-07-20T12:00:00Z",
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
	var pendingErr agentauth.AuthorizationPendingError
	if !errors.As(err, &pendingErr) {
		t.Fatalf("expected pending error, got %v", err)
	}
	if !sawStart {
		t.Fatal("expected start request")
	}
	pending := pendingErr.Pending
	if pending.Method != agentauth.ConsentMethodCodeAuthorization || pending.ChallengeID != "ach_123" || pending.BaseToken != baseToken || pending.CodeVerifier == "" || !pending.UseRefreshTokens {
		t.Fatalf("unexpected pending: %#v", pending)
	}
	for _, want := range []string{"Relay ONLY this URL", "consent_challenge=ach_123", "tollbit auth complete <code>"} {
		if !strings.Contains(inv.stdout.String(), want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, inv.stdout.String())
		}
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
		if body["grant_type"] != "consent:code_authorization" || body["challenge_id"] != "ach_123" || body["code_verifier"] != "verifier" || body["code"] != "K7QF-2MXP" {
			t.Fatalf("unexpected body: %#v", body)
		}
		sawRedeem = true
		_ = json.NewEncoder(w).Encode(auth.AgentTokenResponse{Token: "linked-token"})
	}))
	defer srv.Close()
	strategy := newTestStrategy(t, srv.URL)

	resp, err := strategy.CompleteDetached(&testInvocation{}, testPending(baseToken), agentauth.CompleteDetachedInput{Code: "k7qf 2mxp"})
	if err != nil {
		t.Fatal(err)
	}
	if !sawRedeem || resp.Token != "linked-token" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestNormalizeCode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "display form", input: "K7QF-2MXP", want: "K7QF-2MXP", ok: true},
		{name: "lowercase", input: "k7qf-2mxp", want: "K7QF-2MXP", ok: true},
		{name: "no hyphen", input: "K7QF2MXP", want: "K7QF-2MXP", ok: true},
		{name: "space for hyphen", input: "K7QF 2MXP", want: "K7QF-2MXP", ok: true},
		{name: "surrounding space", input: "  k7qf-2mxp\n", want: "K7QF-2MXP", ok: true},
		{name: "empty", input: ""},
		{name: "too short", input: "K7QF-2MX"},
		{name: "too long", input: "K7QF-2MXPP"},
		{name: "contains zero", input: "K7QF-20XP"},
		{name: "contains one", input: "K7QF-21XP"},
		{name: "contains letter O", input: "K7QF-2OXP"},
		{name: "contains letter I", input: "K7QF-2IXP"},
		{name: "contains letter L", input: "K7QF-2LXP"},
		{name: "contains punctuation", input: "K7QF_2MXP"},
		{name: "non-ascii", input: "K7QF-2MXÉ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeCode(tt.input)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("normalizeCode(%q) = %q, %v; want %q, %v", tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestCompleteDetachedRejectsMalformedCodeWithoutServerCall(t *testing.T) {
	baseToken := testAgentJWT(t)
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer srv.Close()
	strategy := newTestStrategy(t, srv.URL)

	_, err := strategy.CompleteDetached(&testInvocation{}, testPending(baseToken), agentauth.CompleteDetachedInput{Code: "SECRET0-TYPO"})
	if err == nil || !strings.Contains(err.Error(), "malformed confirmation code") || !strings.Contains(err.Error(), "not used up") {
		t.Fatalf("expected malformed code error, got %v", err)
	}
	if strings.Contains(err.Error(), "SECRET0") {
		t.Fatalf("expected error to omit the submitted code, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("expected no server requests, got %d", requests)
	}
}

func TestCompleteDetachedExplainsWhetherTheAttemptSurvived(t *testing.T) {
	tests := []struct {
		code problemjson.ErrorCode
		want string
	}{
		{code: problemjson.ErrorCodeMalformedCode, want: "The attempt was not used up"},
		{code: problemjson.ErrorCodeAccessDenied, want: "this authorization is dead"},
		{code: problemjson.ErrorCodeExpiredToken, want: "expired"},
		{code: problemjson.ErrorCodeInvalidGrant, want: "can no longer be completed"},
	}
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"title": "Bad Request", "status": http.StatusBadRequest, "code": tt.code})
			}))
			defer srv.Close()
			strategy := newTestStrategy(t, srv.URL)

			_, err := strategy.CompleteDetached(&testInvocation{}, testPending(testAgentJWT(t)), agentauth.CompleteDetachedInput{Code: "K7QF-2MXP"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
			var problem problemjson.Problem
			if !errors.As(err, &problem) || problem.Code == nil || *problem.Code != tt.code {
				t.Fatalf("expected wrapped problem with code %q, got %#v", tt.code, problem)
			}
		})
	}
}

func TestCompleteDetachedRejectsOtherPendingMethods(t *testing.T) {
	strategy := newTestStrategy(t, "https://auth.example.test")
	pending := testPending(testAgentJWT(t))
	pending.Method = agentauth.ConsentMethodAgentConfirmsIcons

	_, err := strategy.CompleteDetached(&testInvocation{}, pending, agentauth.CompleteDetachedInput{Code: "K7QF-2MXP"})
	if err == nil || !strings.Contains(err.Error(), "not supported by code-authorization") {
		t.Fatalf("expected unsupported method error, got %v", err)
	}
}

func TestNewConsentStrategyValidatesConfig(t *testing.T) {
	client, err := auth.New(auth.ClientConfig{BaseURL: "https://auth.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewConsentStrategy(ConsentStrategyConfig{Runtime: newTestRuntime(t)}); err == nil {
		t.Fatal("expected missing auth client error")
	}
	if _, err := NewConsentStrategy(ConsentStrategyConfig{AuthClient: client}); err == nil {
		t.Fatal("expected missing runtime error")
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
		Method:        agentauth.ConsentMethodCodeAuthorization,
		ChallengeID:   "ach_123",
		AgentIdentity: auth.AgentIdentity{Name: "agent-test"},
		BaseToken:     baseToken,
		CodeVerifier:  "verifier",
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

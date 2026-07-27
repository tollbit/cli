package browserselecticon

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
	baseToken := testAgentJWT(t, nil)
	var sawStart bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/agent/v1/consent/browser-select-icon/start" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+baseToken {
			t.Fatalf("unexpected authorization: %q", r.Header.Get("Authorization"))
		}
		var body auth.ConsentBrowserSelectIconStartRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.CodeChallenge == "" || body.CodeChallengeMethod != "S256" || body.Scope != "offline_access" {
			t.Fatalf("unexpected body: %#v", body)
		}
		sawStart = true
		_ = json.NewEncoder(w).Encode(auth.ConsentBrowserSelectIconStartResponse{
			ChallengeID: "ach_123",
			ConsentURL:  "https://auth.example.test/oauth/consent/browser-select-icon?consent_challenge=ach_123",
			ExpiresAt:   "2026-07-20T12:00:00Z",
			CorrectIcon: auth.AgentConsentIcon{ID: 17, Name: "ACORN", Art: "ACORN\n art"},
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
		AutoOpenBrowser:  false,
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
	if !sawStart || pending.Pending.ChallengeID != "ach_123" || pending.Pending.CodeVerifier == "" || pending.Pending.CorrectIcon.Name != "ACORN" {
		t.Fatalf("unexpected pending state: %#v", pending.Pending)
	}
	stdout := inv.stdout.String()
	for _, want := range []string{
		"Authorize agent: agent-test",
		"Open this URL in the end user's browser",
		"Relay the following verification icon to the end user exactly",
		"Do not summarize, rename, or replace it with an emoji",
		"The name alone is not sufficient",
		"BEGIN VERIFICATION ICON",
		"Name: ACORN",
		"ACORN\n art",
		"END VERIFICATION ICON",
		"tollbit auth complete",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
		}
	}
	if strategy.AutoOpensBrowser() {
		t.Fatal("browser-select-icon should not auto-open when configured false")
	}
	if strategy.SupportsOBORetry() {
		t.Fatal("browser-select-icon should not support implicit OBO retry")
	}
	if strategy.Guidance().CompleteArgsLabel == "" {
		t.Fatal("expected complete args guidance label")
	}
}

func TestConsentStrategyReportsAutoOpenBrowserConfig(t *testing.T) {
	client, err := auth.New(auth.ClientConfig{BaseURL: "https://auth.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	strategy, err := NewConsentStrategy(ConsentStrategyConfig{
		AuthClient:      client,
		Runtime:         newTestRuntime(t),
		AutoOpenBrowser: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strategy.AutoOpensBrowser() {
		t.Fatal("expected browser-select-icon to report auto-open enabled")
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
		{
			name: "missing auth client",
			config: ConsentStrategyConfig{
				Runtime: newTestRuntime(t),
			},
			wantErr: "auth client is required",
		},
		{
			name: "missing runtime",
			config: ConsentStrategyConfig{
				AuthClient: client,
			},
			wantErr: "runtime is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConsentStrategy(tt.config)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

type testInvocation struct {
	stdout bytes.Buffer
	stderr bytes.Buffer
}

func (i *testInvocation) Context() context.Context {
	return context.Background()
}

func (i *testInvocation) OutOrStdout() io.Writer {
	return &i.stdout
}

func (i *testInvocation) ErrOrStderr() io.Writer {
	return &i.stderr
}

func testAgentJWT(t *testing.T, obo *agent.OBOClaims) string {
	t.Helper()
	claims := agent.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "agent-test",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		TBT: "agent-token",
		OBO: obo,
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

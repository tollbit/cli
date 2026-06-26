package agentauth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tollbit/tollbit-cli/internal/client/auth"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
)

func TestBrowserConsentAuthorizerAuthorizesOBO(t *testing.T) {
	baseToken := testAgentJWT(t, nil)
	oboToken := testAgentJWT(t, &agent.OBOClaims{Ver: 1, Source: "consent", User: "usr_abc", Org: "org_xyz"})
	callbackCh := make(chan string, 1)
	var sawStart bool
	var sawRedeem bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/agent/v1/consent/redirect/start":
			if r.Header.Get("Authorization") != "Bearer "+baseToken {
				t.Fatalf("unexpected start authorization: %q", r.Header.Get("Authorization"))
			}
			var body auth.ConsentRedirectStartRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.RedirectURI == "" || body.State == "" || body.CodeChallenge == "" || body.CodeChallengeMethod != "S256" {
				t.Fatalf("unexpected start body: %#v", body)
			}
			sawStart = true
			go func() {
				callbackURL, err := url.Parse(body.RedirectURI)
				if err != nil {
					t.Errorf("parse redirect uri: %v", err)
					return
				}
				query := callbackURL.Query()
				query.Set("code", "code-123")
				query.Set("state", body.State)
				callbackURL.RawQuery = query.Encode()
				callbackCh <- callbackURL.String()
			}()
			_ = json.NewEncoder(w).Encode(map[string]string{"consent_url": "https://auth.example.test/consent"})
		case "/agent/v1/consent/redirect/token":
			if r.Header.Get("Authorization") != "Bearer "+baseToken {
				t.Fatalf("unexpected redeem authorization: %q", r.Header.Get("Authorization"))
			}
			var body auth.ConsentRedirectTokenRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Code != "code-123" || body.CodeVerifier == "" || body.RedirectURI == "" {
				t.Fatalf("unexpected redeem body: %#v", body)
			}
			sawRedeem = true
			_ = json.NewEncoder(w).Encode(map[string]string{"token": oboToken})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	authClient, err := auth.New(auth.ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	authorizer, err := NewBrowserConsentAuthorizer(BrowserConsentAuthorizerConfig{
		AuthClient:      authClient,
		CallbackAddress: "127.0.0.1:54321",
		AutoOpenBrowser: false,
		Timeout:         2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	inv := &testInvocation{}

	go func() {
		callbackURL := <-callbackCh
		_, _ = http.Get(callbackURL)
	}()
	got, err := authorizer.AuthorizeOBO(inv, auth.AgentIdentity{Name: "agent-test"}, agent.Token{RawToken: baseToken})
	if err != nil {
		t.Fatal(err)
	}
	if got.RawToken != oboToken {
		t.Fatalf("expected OBO token, got %q", got.RawToken)
	}
	if !sawStart || !sawRedeem {
		t.Fatalf("expected start and redeem, sawStart=%v sawRedeem=%v", sawStart, sawRedeem)
	}
	stdout := inv.stdout.String()
	for _, want := range []string{"Authorize agent: agent-test", "Open this URL in your browser", "Waiting for authorization"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
		}
	}
}

func TestNewBrowserConsentAuthorizerValidatesConfig(t *testing.T) {
	authClient := newTestAuthClient(t)
	tests := []struct {
		name    string
		config  BrowserConsentAuthorizerConfig
		wantErr string
	}{
		{
			name: "missing auth client",
			config: BrowserConsentAuthorizerConfig{
				CallbackAddress: "127.0.0.1:54321",
			},
			wantErr: "auth client is required",
		},
		{
			name: "missing callback address",
			config: BrowserConsentAuthorizerConfig{
				AuthClient: authClient,
			},
			wantErr: "callback address is required",
		},
		{
			name: "negative timeout",
			config: BrowserConsentAuthorizerConfig{
				AuthClient:      authClient,
				CallbackAddress: "127.0.0.1:54321",
				Timeout:         -time.Second,
			},
			wantErr: "timeout must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewBrowserConsentAuthorizer(tt.config)
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

func newTestAuthClient(t *testing.T) *auth.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(srv.Close)
	c, err := auth.New(auth.ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

package agenttoken

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tollbit/tollbit-cli/internal/agentauth"
	"github.com/tollbit/tollbit-cli/internal/client/auth"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
)

func TestGetAgentTokenReturnsValidTokenFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, tokenFilename)
	storedToken := testJWT(t, validClaims())
	if err := os.WriteFile(path, []byte(storedToken), 0o600); err != nil {
		t.Fatal(err)
	}

	mgr := newTestManager(t, dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("auth client should not be called for valid cached token")
	}))

	token, err := mgr.GetAgentToken(testInvocation{}, auth.AgentIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	if token.RawToken != storedToken {
		t.Fatalf("expected cached token, got %q", token.RawToken)
	}
}

func TestGetAgentTokenMintsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, tokenFilename)
	mintedToken := testJWT(t, validClaims())
	var mintCount int

	mgr := newTestManager(t, dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mintCount++
		_ = json.NewEncoder(w).Encode(map[string]string{"token": mintedToken})
	}))

	token, err := mgr.GetAgentToken(testInvocation{}, testAgentIdentity())
	if err != nil {
		t.Fatal(err)
	}
	if token.RawToken != mintedToken {
		t.Fatalf("expected minted token, got %q", token.RawToken)
	}
	if mintCount != 1 {
		t.Fatalf("expected one mint, got %d", mintCount)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != mintedToken {
		t.Fatalf("expected token written to disk")
	}
}

func TestGetAgentTokenUsesConfiguredTTL(t *testing.T) {
	mintedToken := testJWT(t, validClaims())
	ttl := int32(120)
	mgr := newTestManagerWithConfig(t, t.TempDir(), CredentialManagerConfig{
		TokenOptions: auth.AgentTokenOptions{TTLSeconds: &ttl},
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body auth.AgentTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.TTLSeconds == nil || *body.TTLSeconds != 120 {
			t.Fatalf("expected ttl 120, got %#v", body.TTLSeconds)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": mintedToken})
	}))

	if _, err := mgr.GetAgentToken(testInvocation{}, testAgentIdentity()); err != nil {
		t.Fatal(err)
	}
}

func TestGetAgentTokenRequiresIdentityWhenMissing(t *testing.T) {
	dir := t.TempDir()
	mgr := newTestManager(t, dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("auth service should reject before request is made")
	}))

	_, err := mgr.GetAgentToken(testInvocation{}, auth.AgentIdentity{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "agent name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetAgentTokenFetchesNewToken(t *testing.T) {
	tests := []struct {
		name string
		raw  func(t *testing.T) string
	}{
		{
			name: "expired token",
			raw: func(t *testing.T) string {
				return testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
					claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
					return claims
				}))
			},
		},
		{
			name: "malformed token",
			raw: func(t *testing.T) string {
				return "not-a-jwt"
			},
		},
		{
			name: "wrong tbt",
			raw: func(t *testing.T) string {
				return testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
					claims.TBT = "oauth"
					return claims
				}))
			},
		},
		{
			name: "future nbf",
			raw: func(t *testing.T) string {
				return testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
					claims.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Hour))
					return claims
				}))
			},
		},
		{
			name: "missing exp",
			raw: func(t *testing.T) string {
				return testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
					claims.ExpiresAt = nil
					return claims
				}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tokenFilename)
			if err := os.WriteFile(path, []byte(tt.raw(t)), 0o600); err != nil {
				t.Fatal(err)
			}
			mintedToken := testJWT(t, validClaims())
			var mintCount int

			mgr := newTestManager(t, dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mintCount++
				_ = json.NewEncoder(w).Encode(map[string]string{"token": mintedToken})
			}))

			token, err := mgr.GetAgentToken(testInvocation{}, testAgentIdentity())
			if err != nil {
				t.Fatal(err)
			}
			if token.RawToken != mintedToken {
				t.Fatalf("expected minted token, got %q", token.RawToken)
			}
			if mintCount != 1 {
				t.Fatalf("expected one mint, got %d", mintCount)
			}
			written, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(written) != mintedToken {
				t.Fatalf("expected minted token written to disk")
			}
		})
	}
}

func TestClearRemovesToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, tokenFilename)
	if err := os.WriteFile(path, []byte(testJWT(t, validClaims())), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr := newTestManager(t, dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	if err := mgr.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected token file removed, got err=%v", err)
	}
}

func TestClearAuthTokensRemovesAgentAndRefreshTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, tokenFilename)
	if err := os.WriteFile(path, []byte(testJWT(t, validClaims())), 0o600); err != nil {
		t.Fatal(err)
	}
	var sawRevoke bool
	mgr := newTestManager(t, dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/agent/v1/tokens/refresh/revoke" {
			t.Fatalf("unexpected auth request: %s %s", r.Method, r.URL.Path)
		}
		var body auth.RevokeRefreshTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.RefreshToken != "agrt_old" {
			t.Fatalf("unexpected revoke body: %#v", body)
		}
		sawRevoke = true
		_ = json.NewEncoder(w).Encode(auth.RevokeRefreshTokenResponse{Revoked: true})
	}))
	if err := mgr.writeJSON(context.Background(), mgr.refreshPath, refreshCredential{RefreshToken: "agrt_old"}); err != nil {
		t.Fatal(err)
	}

	if err := mgr.ClearAuthTokens(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected token file removed, got err=%v", err)
	}
	if !sawRevoke {
		t.Fatal("expected refresh token revocation")
	}
	if _, err := os.Stat(mgr.refreshPath); !os.IsNotExist(err) {
		t.Fatalf("expected refresh token file removed, got err=%v", err)
	}
}

func TestClearAuthTokensKeepsRefreshTokenWhenRevokeFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, tokenFilename)
	if err := os.WriteFile(path, []byte(testJWT(t, validClaims())), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr := newTestManager(t, dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/agent/v1/tokens/refresh/revoke" {
			t.Fatalf("unexpected auth request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"detail": "revoke failed"})
	}))
	if err := mgr.writeJSON(context.Background(), mgr.refreshPath, refreshCredential{RefreshToken: "agrt_old"}); err != nil {
		t.Fatal(err)
	}

	if err := mgr.ClearAuthTokens(context.Background()); err == nil {
		t.Fatal("expected revoke error")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected agent token to remain after revoke failure, got err=%v", err)
	}
	if _, err := os.Stat(mgr.refreshPath); err != nil {
		t.Fatalf("expected refresh token to remain after revoke failure, got err=%v", err)
	}
}

func TestClearSucceedsWhenTokenIsMissing(t *testing.T) {
	dir := t.TempDir()
	mgr := newTestManager(t, dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	if err := mgr.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestGetAgentTokenWithOBOReturnsCachedOBOToken(t *testing.T) {
	dir := t.TempDir()
	raw := testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", User: "usr_123", Org: "org_456"}
		return claims
	}))
	if err := os.WriteFile(filepath.Join(dir, tokenFilename), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr := newTestManager(t, dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("auth client should not be called for cached OBO token")
	}))

	got, err := mgr.GetAgentToken(testInvocation{}, testAgentIdentity(), WithOBO())
	if err != nil {
		t.Fatal(err)
	}
	if got.RawToken != raw {
		t.Fatalf("expected cached OBO token, got %q", got.RawToken)
	}
	claims, err := got.Claims()
	if err != nil {
		t.Fatal(err)
	}
	if claims.OBO == nil || claims.OBO.User != "usr_123" || claims.OBO.Org != "org_456" {
		t.Fatalf("unexpected OBO claims: %#v", claims.OBO)
	}
	info, err := os.Stat(filepath.Join(dir, tokenFilename))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected token file mode 0600, got %#o", got)
	}
}

func TestGetAgentTokenWithOBOAuthorizesAndOverwritesToken(t *testing.T) {
	dir := t.TempDir()
	baseToken := testJWT(t, validClaims())
	oboToken := testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", User: "usr_abc", Org: "org_xyz"}
		return claims
	}))
	if err := os.WriteFile(filepath.Join(dir, tokenFilename), []byte(baseToken), 0o600); err != nil {
		t.Fatal(err)
	}

	authorizer := &fakeOBOAuthorizer{token: oboToken}
	mgr := newTestManagerWithConfig(t, dir, CredentialManagerConfig{OBOAuthorizer: authorizer}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("auth client should not be called when base token is cached")
	}))

	got, err := mgr.GetAgentToken(testInvocation{}, testAgentIdentity(), WithOBO())
	if err != nil {
		t.Fatal(err)
	}
	if authorizer.callCount != 1 || authorizer.baseToken.RawToken != baseToken {
		t.Fatalf("unexpected authorizer call count=%d base=%q", authorizer.callCount, authorizer.baseToken.RawToken)
	}
	if got.RawToken != oboToken {
		t.Fatalf("expected OBO token, got %q", got.RawToken)
	}
	written, err := os.ReadFile(filepath.Join(dir, tokenFilename))
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != oboToken {
		t.Fatalf("expected OBO token written to canonical token file")
	}
}

func TestGetAgentTokenWithOBOMintsBaseTokenBeforeAuthorizing(t *testing.T) {
	dir := t.TempDir()
	baseToken := testJWT(t, validClaims())
	oboToken := testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", User: "usr_abc", Org: "org_xyz"}
		return claims
	}))
	authorizer := &fakeOBOAuthorizer{token: oboToken}
	mgr := newTestManagerWithConfig(t, dir, CredentialManagerConfig{OBOAuthorizer: authorizer}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": baseToken})
	}))

	got, err := mgr.GetAgentToken(testInvocation{}, testAgentIdentity(), WithOBO())
	if err != nil {
		t.Fatal(err)
	}
	if got.RawToken != oboToken || authorizer.baseToken.RawToken != baseToken {
		t.Fatalf("expected minted base token to authorize OBO, got token=%q base=%q", got.RawToken, authorizer.baseToken.RawToken)
	}
}

func TestNewRequiresOBOAuthorizer(t *testing.T) {
	_, err := New(CredentialManagerConfig{
		Path:            t.TempDir(),
		DefaultIdentity: auth.AgentIdentity{Name: "anonymous"},
		AuthClient:      newTestAuthClient(t),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "OBO authorizer is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetAgentTokenWithOBORefreshesExpiredTokenAndStoresRotatedRefreshToken(t *testing.T) {
	dir := t.TempDir()
	expiredOBO := testJWT(t, expiredClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", User: "usr_old", Org: "org_old"}
		return claims
	}))
	baseToken := testJWT(t, validClaims())
	refreshedOBO := testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", User: "usr_new", Org: "org_new"}
		return claims
	}))

	mgr := newTestManagerWithConfig(t, dir, CredentialManagerConfig{UseRefreshTokens: true, OBOAuthorizer: &fakeOBOAuthorizer{}}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		switch body["grant_type"] {
		case "self_attested":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": baseToken})
		case "refresh_token":
			if r.Header.Get("Authorization") != "Bearer "+baseToken {
				t.Fatalf("unexpected refresh authorization: %q", r.Header.Get("Authorization"))
			}
			if body["agent_identifier"] != "agent-test" || body["refresh_token"] != "agrt_old" {
				t.Fatalf("unexpected refresh body: %#v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"token":                    refreshedOBO,
				"refresh_token":            "agrt_new",
				"refresh_token_expires_at": "2026-07-08T12:00:00Z",
			})
		default:
			t.Fatalf("unexpected grant body: %#v", body)
		}
	}))
	if err := os.WriteFile(filepath.Join(dir, tokenFilename), []byte(expiredOBO), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.writeJSON(context.Background(), mgr.refreshPath, refreshCredential{RefreshToken: "agrt_old"}); err != nil {
		t.Fatal(err)
	}

	got, err := mgr.GetAgentToken(testInvocation{}, testAgentIdentity(), WithOBO())
	if err != nil {
		t.Fatal(err)
	}
	if got.RawToken != refreshedOBO {
		t.Fatalf("expected refreshed OBO token, got %q", got.RawToken)
	}
	refresh, exists, err := mgr.readRefreshCredential(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !exists || refresh.RefreshToken != "agrt_new" || refresh.RefreshTokenExpiresAt != "2026-07-08T12:00:00Z" {
		t.Fatalf("unexpected refresh credential exists=%v cred=%#v", exists, refresh)
	}
}

func TestGetAgentTokenWithOBOClearsFailedRefreshTokenAndFallsBackToConsent(t *testing.T) {
	dir := t.TempDir()
	expiredOBO := testJWT(t, expiredClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", User: "usr_old", Org: "org_old"}
		return claims
	}))
	baseToken := testJWT(t, validClaims())
	consentOBO := testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", User: "usr_new", Org: "org_new"}
		return claims
	}))
	authorizer := &fakeOBOAuthorizer{token: consentOBO}
	var refreshCount int

	mgr := newTestManagerWithConfig(t, dir, CredentialManagerConfig{UseRefreshTokens: true, OBOAuthorizer: authorizer}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		switch body["grant_type"] {
		case "self_attested":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": baseToken})
		case "refresh_token":
			refreshCount++
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"detail": "refresh token invalid"})
		default:
			t.Fatalf("unexpected grant body: %#v", body)
		}
	}))
	if err := os.WriteFile(filepath.Join(dir, tokenFilename), []byte(expiredOBO), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.writeJSON(context.Background(), mgr.refreshPath, refreshCredential{RefreshToken: "agrt_old"}); err != nil {
		t.Fatal(err)
	}

	got, err := mgr.GetAgentToken(testInvocation{}, testAgentIdentity(), WithOBO())
	if err != nil {
		t.Fatal(err)
	}
	if got.RawToken != consentOBO {
		t.Fatalf("expected consent fallback OBO token, got %q", got.RawToken)
	}
	if refreshCount != 1 || authorizer.callCount != 1 {
		t.Fatalf("expected one refresh attempt and one consent fallback, refresh=%d consent=%d", refreshCount, authorizer.callCount)
	}
	if _, exists, err := mgr.readRefreshCredential(context.Background()); err != nil || exists {
		t.Fatalf("expected failed refresh token to be cleared, exists=%v err=%v", exists, err)
	}
}

func TestGetAgentTokenWithOBOClearsExpiredRefreshTokenAndFallsBackToConsent(t *testing.T) {
	dir := t.TempDir()
	expiredOBO := testJWT(t, expiredClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", User: "usr_old", Org: "org_old"}
		return claims
	}))
	baseToken := testJWT(t, validClaims())
	consentOBO := testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", User: "usr_new", Org: "org_new"}
		return claims
	}))
	authorizer := &fakeOBOAuthorizer{token: consentOBO}
	var refreshCount int

	mgr := newTestManagerWithConfig(t, dir, CredentialManagerConfig{UseRefreshTokens: true, OBOAuthorizer: authorizer}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		switch body["grant_type"] {
		case "self_attested":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": baseToken})
		case "refresh_token":
			refreshCount++
			t.Fatal("refresh token grant should not be called for expired refresh token")
		default:
			t.Fatalf("unexpected grant body: %#v", body)
		}
	}))
	if err := os.WriteFile(filepath.Join(dir, tokenFilename), []byte(expiredOBO), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.writeJSON(context.Background(), mgr.refreshPath, refreshCredential{RefreshToken: "agrt_old", RefreshTokenExpiresAt: "2000-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}

	got, err := mgr.GetAgentToken(testInvocation{}, testAgentIdentity(), WithOBO())
	if err != nil {
		t.Fatal(err)
	}
	if got.RawToken != consentOBO {
		t.Fatalf("expected consent fallback OBO token, got %q", got.RawToken)
	}
	if refreshCount != 0 || authorizer.callCount != 1 {
		t.Fatalf("expected no refresh attempt and one consent fallback, refresh=%d consent=%d", refreshCount, authorizer.callCount)
	}
	if _, exists, err := mgr.readRefreshCredential(context.Background()); err != nil || exists {
		t.Fatalf("expected expired refresh token to be cleared, exists=%v err=%v", exists, err)
	}
}

func TestCanonicalAgentTokenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	mgr, err := New(CredentialManagerConfig{Path: dir, DefaultIdentity: auth.AgentIdentity{Name: "anonymous"}, AuthClient: newTestAuthClient(t), OBOAuthorizer: &fakeOBOAuthorizer{}})
	if err != nil {
		t.Fatal(err)
	}
	raw := testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", User: "usr_123", Org: "org_456"}
		return claims
	}))

	if err := mgr.write(context.Background(), agent.Token{RawToken: raw}); err != nil {
		t.Fatal(err)
	}
	got, exists, err := mgr.CurrentAgentToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !exists || got.RawToken != raw {
		t.Fatalf("unexpected token exists=%v token=%q", exists, got.RawToken)
	}
	info, err := os.Stat(filepath.Join(dir, tokenFilename))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected token file mode 0600, got %#o", got)
	}

	if err := mgr.ClearAgentTokens(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := mgr.CurrentAgentToken(context.Background()); err != nil || exists {
		t.Fatalf("expected token cleared, exists=%v err=%v", exists, err)
	}
}

func TestCurrentAgentTokenDoesNotMint(t *testing.T) {
	mgr := newTestManager(t, t.TempDir(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("auth client should not be called for cached lookup")
	}))
	token, exists, err := mgr.CurrentAgentToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if exists || token.RawToken != "" {
		t.Fatalf("expected missing cached token, exists=%v token=%q", exists, token.RawToken)
	}
}

func TestCurrentAgentTokenReturnsInvalidTokenError(t *testing.T) {
	dir := t.TempDir()
	raw := testJWT(t, expiredClaims())
	if err := os.WriteFile(filepath.Join(dir, tokenFilename), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr := newTestManager(t, dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("auth client should not be called for current token lookup")
	}))

	token, exists, err := mgr.CurrentAgentToken(context.Background())
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !exists {
		t.Fatal("expected current token to exist")
	}
	if token.RawToken != raw {
		t.Fatalf("expected current token, got %q", token.RawToken)
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired token error, got %v", err)
	}
}

func TestGetAgentTokenWritesSecureFileOverStaleTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, tokenFilename)
	staleTemp := path + ".tmp"
	if err := os.WriteFile(staleTemp, []byte("stale"), 0o666); err != nil {
		t.Fatal(err)
	}
	mintedToken := testJWT(t, validClaims())
	mgr := newTestManager(t, dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": mintedToken})
	}))

	if _, err := mgr.GetAgentToken(testInvocation{}, testAgentIdentity()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected token file mode 0600, got %#o", got)
	}
	stale, err := os.ReadFile(staleTemp)
	if err != nil {
		t.Fatal(err)
	}
	if string(stale) != "stale" {
		t.Fatalf("expected stale deterministic temp file to remain untouched")
	}
}

func TestNewRequiresAuthClient(t *testing.T) {
	_, err := New(CredentialManagerConfig{Path: t.TempDir()})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "auth client is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewTreatsPathAsDirectory(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c, err := auth.New(auth.ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	mgr, err := New(CredentialManagerConfig{Path: dir, DefaultIdentity: auth.AgentIdentity{Name: "anonymous"}, AuthClient: c, OBOAuthorizer: &fakeOBOAuthorizer{}})
	if err != nil {
		t.Fatal(err)
	}
	if mgr.dir != dir {
		t.Fatalf("expected dir %q, got %q", dir, mgr.dir)
	}
	if want := filepath.Join(dir, tokenFilename); mgr.tokenPath != want {
		t.Fatalf("expected token path %q, got %q", want, mgr.tokenPath)
	}
}

func TestIdentityCredentialRoundTrip(t *testing.T) {
	dir := t.TempDir()
	mgr, err := New(CredentialManagerConfig{Path: dir, DefaultIdentity: auth.AgentIdentity{Name: "anonymous"}, AuthClient: newTestAuthClient(t), OBOAuthorizer: &fakeOBOAuthorizer{}})
	if err != nil {
		t.Fatal(err)
	}
	identity := auth.AgentIdentity{
		Name:      " agent-test ",
		UserAgent: " agent-test/0.1 ",
		WBA:       &auth.WebBotAuth{Dir: "https://example.com/.well-known/web-bot-auth", Req: true, Ver: 1},
	}

	if err := mgr.SaveIdentity(context.Background(), identity); err != nil {
		t.Fatal(err)
	}
	got, exists, err := mgr.GetStoredIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected stored identity")
	}
	if got.Name != "agent-test" || got.UserAgent != "agent-test/0.1" || got.WBA == nil || got.WBA.Ver != 1 {
		t.Fatalf("unexpected identity: %#v", got)
	}
	info, err := os.Stat(filepath.Join(dir, identityFilename))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected identity file mode 0600, got %#o", got)
	}
}

func TestGetStoredIdentityReturnsFalseWhenMissing(t *testing.T) {
	mgr, err := New(CredentialManagerConfig{Path: t.TempDir(), DefaultIdentity: auth.AgentIdentity{Name: "anonymous"}, AuthClient: newTestAuthClient(t), OBOAuthorizer: &fakeOBOAuthorizer{}})
	if err != nil {
		t.Fatal(err)
	}
	identity, exists, err := mgr.GetStoredIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("expected no stored identity, got %#v", identity)
	}
}

func TestGetIdentityUsesConfiguredDefaultWhenMissing(t *testing.T) {
	mgr, err := New(CredentialManagerConfig{
		Path:            t.TempDir(),
		DefaultIdentity: auth.AgentIdentity{Name: "configured-agent", UserAgent: "configured-agent/0.1"},
		AuthClient:      newTestAuthClient(t),
		OBOAuthorizer:   &fakeOBOAuthorizer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := mgr.GetIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if identity.Name != "configured-agent" || identity.UserAgent != "configured-agent/0.1" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestResolveIdentityUsesConfiguredDefaultWhenMissing(t *testing.T) {
	mgr, err := New(CredentialManagerConfig{
		Path:            t.TempDir(),
		DefaultIdentity: auth.AgentIdentity{Name: " configured-agent ", UserAgent: " configured-agent/0.1 "},
		AuthClient:      newTestAuthClient(t),
		OBOAuthorizer:   &fakeOBOAuthorizer{},
	})
	if err != nil {
		t.Fatal(err)
	}

	identity, err := mgr.ResolveIdentity(context.Background(), ResolveIdentityOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if identity.Name != "configured-agent" || identity.UserAgent != "configured-agent/0.1" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestResolveIdentityUsesStoredIdentity(t *testing.T) {
	mgr, err := New(CredentialManagerConfig{
		Path:            t.TempDir(),
		DefaultIdentity: auth.AgentIdentity{Name: "configured-agent", UserAgent: "configured-agent/0.1"},
		AuthClient:      newTestAuthClient(t),
		OBOAuthorizer:   &fakeOBOAuthorizer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.SaveIdentity(context.Background(), auth.AgentIdentity{Name: " stored-agent ", UserAgent: " stored-agent/0.1 "}); err != nil {
		t.Fatal(err)
	}

	identity, err := mgr.ResolveIdentity(context.Background(), ResolveIdentityOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if identity.Name != "stored-agent" || identity.UserAgent != "stored-agent/0.1" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestResolveIdentityNameOverrideWinsOverStoredIdentity(t *testing.T) {
	mgr, err := New(CredentialManagerConfig{Path: t.TempDir(), DefaultIdentity: auth.AgentIdentity{Name: "configured-agent"}, AuthClient: newTestAuthClient(t), OBOAuthorizer: &fakeOBOAuthorizer{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.SaveIdentity(context.Background(), auth.AgentIdentity{Name: "stored-agent", UserAgent: "stored-agent/0.1"}); err != nil {
		t.Fatal(err)
	}
	name := " explicit-agent "

	identity, err := mgr.ResolveIdentity(context.Background(), ResolveIdentityOptions{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if identity.Name != "explicit-agent" || identity.UserAgent != "stored-agent/0.1" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestResolveIdentityUserAgentOverrideWinsOverStoredIdentity(t *testing.T) {
	mgr, err := New(CredentialManagerConfig{Path: t.TempDir(), DefaultIdentity: auth.AgentIdentity{Name: "configured-agent"}, AuthClient: newTestAuthClient(t), OBOAuthorizer: &fakeOBOAuthorizer{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.SaveIdentity(context.Background(), auth.AgentIdentity{Name: "stored-agent", UserAgent: "stored-agent/0.1"}); err != nil {
		t.Fatal(err)
	}
	userAgent := " explicit-agent/0.1 "

	identity, err := mgr.ResolveIdentity(context.Background(), ResolveIdentityOptions{UserAgent: &userAgent})
	if err != nil {
		t.Fatal(err)
	}
	if identity.Name != "stored-agent" || identity.UserAgent != "explicit-agent/0.1" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestResolveIdentityRequiresName(t *testing.T) {
	mgr, err := New(CredentialManagerConfig{Path: t.TempDir(), DefaultIdentity: auth.AgentIdentity{Name: "configured-agent"}, AuthClient: newTestAuthClient(t), OBOAuthorizer: &fakeOBOAuthorizer{}})
	if err != nil {
		t.Fatal(err)
	}
	name := " \t "

	_, err = mgr.ResolveIdentity(context.Background(), ResolveIdentityOptions{Name: &name})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "agent name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSaveIdentityClearsToken(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, tokenFilename)
	if err := os.WriteFile(tokenPath, []byte(testJWT(t, validClaims())), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr, err := New(CredentialManagerConfig{Path: dir, DefaultIdentity: auth.AgentIdentity{Name: "anonymous"}, AuthClient: newTestAuthClient(t), OBOAuthorizer: &fakeOBOAuthorizer{}})
	if err != nil {
		t.Fatal(err)
	}

	if err := mgr.SaveIdentity(context.Background(), testAgentIdentity()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Fatalf("expected token to be cleared, got err=%v", err)
	}
}

func TestClearIdentityRemovesIdentityAndToken(t *testing.T) {
	dir := t.TempDir()
	mgr, err := New(CredentialManagerConfig{Path: dir, DefaultIdentity: auth.AgentIdentity{Name: "anonymous"}, AuthClient: newTestAuthClient(t), OBOAuthorizer: &fakeOBOAuthorizer{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.SaveIdentity(context.Background(), testAgentIdentity()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, tokenFilename), []byte(testJWT(t, validClaims())), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.ClearIdentity(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, identityFilename)); !os.IsNotExist(err) {
		t.Fatalf("expected identity to be cleared, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, tokenFilename)); !os.IsNotExist(err) {
		t.Fatalf("expected token to be cleared, got err=%v", err)
	}
}

func newTestManager(t *testing.T, dir string, handler http.Handler) *CredentialManager {
	t.Helper()
	return newTestManagerWithConfig(t, dir, CredentialManagerConfig{}, handler)
}

func newTestManagerWithConfig(t *testing.T, dir string, cfg CredentialManagerConfig, handler http.Handler) *CredentialManager {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := auth.New(auth.ClientConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Path = dir
	if strings.TrimSpace(cfg.DefaultIdentity.Name) == "" {
		cfg.DefaultIdentity = auth.AgentIdentity{Name: "anonymous"}
	}
	cfg.AuthClient = c
	if cfg.OBOAuthorizer == nil {
		cfg.OBOAuthorizer = &fakeOBOAuthorizer{}
	}
	mgr, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return mgr
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

type testInvocation struct {
	ctx    context.Context
	stdout bytes.Buffer
	stderr bytes.Buffer
}

func (i testInvocation) Context() context.Context {
	if i.ctx != nil {
		return i.ctx
	}
	return context.Background()
}

func (i testInvocation) OutOrStdout() io.Writer {
	return &i.stdout
}

func (i testInvocation) ErrOrStderr() io.Writer {
	return &i.stderr
}

type fakeOBOAuthorizer struct {
	token     string
	baseToken agent.Token
	callCount int
}

func (a *fakeOBOAuthorizer) AuthorizeOBO(inv agentauth.Invocation, identity auth.AgentIdentity, baseToken agent.Token) (auth.AgentTokenResponse, error) {
	a.callCount++
	a.baseToken = baseToken
	return auth.AgentTokenResponse{Token: a.token}, nil
}

func testAgentIdentity() auth.AgentIdentity {
	return auth.AgentIdentity{Name: "agent-test", UserAgent: "agent-test/0.1"}
}

func validClaims(mutators ...func(agent.Claims) agent.Claims) agent.Claims {
	claims := agent.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://oauth.tollbit.com",
			Audience:  jwt.ClaimStrings{"tollbit.com"},
			Subject:   "agent-test",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			ID:        "agtok_test",
		},
		TBT: "agent-token",
	}
	for _, mutate := range mutators {
		claims = mutate(claims)
	}
	return claims
}

func expiredClaims(mutators ...func(agent.Claims) agent.Claims) agent.Claims {
	mutators = append([]func(agent.Claims) agent.Claims{func(c agent.Claims) agent.Claims {
		c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
		c.IssuedAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
		c.NotBefore = jwt.NewNumericDate(time.Now().Add(-time.Minute))

		return c
	}}, mutators...)
	return validClaims(mutators...)
}

func testJWT(t *testing.T, claims agent.Claims) string {
	t.Helper()
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

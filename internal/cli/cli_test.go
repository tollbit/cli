package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/app"
	"github.com/tollbit/cli/internal/client/auth"
	"github.com/tollbit/cli/internal/client/tollbit"
	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/version"
)

const testGatewayBaseURLEnvVar = "TOLLBIT_GATEWAY_BASE_URL"
const testAuthBaseURLEnvVar = "TOLLBIT_AUTH_BASE_URL"
const testCredentialsStorageDirEnvVar = "TOLLBIT_CREDENTIALS_STORAGE_DIR"
const testAgentDefaultNameEnvVar = "TOLLBIT_AGENT_DEFAULT_NAME"
const testAgentDefaultUserAgentEnvVar = "TOLLBIT_AGENT_DEFAULT_USER_AGENT"

func executeTestCommand(args []string, stdin io.Reader, stdout, stderr *bytes.Buffer) int {
	cmd := NewCommandTree(app.Factory{Config: testConfig()})
	cmd.SetArgs(args)
	cmd.SetIn(stdin)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	err := cmd.Execute()
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
	return ExitCode(err)
}

func testConfig() configuration.Config {
	gatewayBaseURL := os.Getenv(testGatewayBaseURLEnvVar)
	if gatewayBaseURL == "" {
		gatewayBaseURL = "https://gateway.tollbit.com"
	}
	authBaseURL := os.Getenv(testAuthBaseURLEnvVar)
	if authBaseURL == "" {
		authBaseURL = "https://oauth.tollbit.com"
	}
	storageDir := os.Getenv(testCredentialsStorageDirEnvVar)
	if storageDir == "" {
		storageDir = filepath.Join(os.TempDir(), "tollbit-cli-test-credentials")
	}
	agentDefaultName := os.Getenv(testAgentDefaultNameEnvVar)
	if agentDefaultName == "" {
		agentDefaultName = "anonymous"
	}
	agentDefaultUserAgent := os.Getenv(testAgentDefaultUserAgentEnvVar)
	return configuration.Config{
		App: configuration.AppConfig{
			Name: "tollbit",
		},
		Runtime: configuration.RuntimeConfig{EndUserProximity: configuration.RuntimeEndUserProximityLocal, StateDir: storageDir},
		Auth: configuration.AuthConfig{
			BaseURL:          authBaseURL,
			UseRefreshTokens: true,
			Consent: configuration.ConsentConfig{
				Strategy: configuration.ConsentStrategyConfig{
					Local:  configuration.ConsentStrategyRedirect,
					Remote: configuration.ConsentStrategyBrowserSelectIcon,
				},
			},
			BrowserConsent: configuration.BrowserConsentConfig{
				CallbackAddress: "127.0.0.1:54321",
				Timeout:         3 * time.Minute,
				AutoOpenBrowser: false,
			},
		},
		Agent: configuration.AgentConfig{
			DefaultName:          agentDefaultName,
			DefaultUserAgent:     agentDefaultUserAgent,
			RegisterUserAgentURL: "https://hack.tollbit.com/my-agents",
		},
		Credentials: configuration.CredentialsConfig{StorageDir: storageDir},
		Gateway:     configuration.GatewayConfig{BaseURL: gatewayBaseURL},
	}
}

func TestRunSearchRendersResults(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.RequestURI() != "/agent/v1/tokens/identity" {
			t.Fatalf("unexpected auth request: %s %s", r.Method, r.URL.RequestURI())
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/agents/v1/search" {
			t.Fatalf("unexpected gateway request: %s %s", r.Method, r.URL.String())
		}
		if r.URL.Query().Get("q") != "climate policy" {
			t.Fatalf("unexpected q: %q", r.URL.Query().Get("q"))
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(tollbit.PagedSearchResultResponse{
			NextToken: "page-2",
			Items: []tollbit.SearchResult{{
				Title:         "Climate Policy Overview",
				URL:           "https://example.com/climate",
				PublishedDate: "2024-06-01",
				Publisher:     tollbit.Publisher{Domain: "example.com", Name: "Example News"},
				Availability:  tollbit.Availability{Discoverable: true, ReadyToLicense: true},
			}},
		})
	}))
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"search", "climate policy", "--size", "5"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if want := "Climate Policy Overview"; !strings.Contains(stdout.String(), want) {
		t.Fatalf("expected stdout to contain %q, got %q", want, stdout.String())
	}
	if want := "In TollBit network"; !strings.Contains(stdout.String(), want) {
		t.Fatalf("expected stdout to contain %q, got %q", want, stdout.String())
	}
	if want := "Programmatic"; !strings.Contains(stdout.String(), want) {
		t.Fatalf("expected stdout to contain %q, got %q", want, stdout.String())
	}
	if want := "page-2"; !strings.Contains(stdout.String(), want) {
		t.Fatalf("expected stdout to contain next-token hint, got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "To get pricing:") {
		t.Fatalf("expected pricing leading command on stderr, not stdout; got %q", stdout.String())
	}
	if want := "To get pricing: tollbit content pricing <url>[,<url>...]"; !strings.Contains(stderr.String(), want) {
		t.Fatalf("expected stderr to contain pricing leading command, got %q", stderr.String())
	}
}

func TestRunSearchJSON(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tollbit.PagedSearchResultResponse{
			Items: []tollbit.SearchResult{{
				Title: "Result",
				URL:   "https://example.com/a",
			}},
		})
	}))
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"search", "test", "--json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "To get pricing:") {
		t.Fatalf("expected no leading command in --json output, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "To get pricing:") {
		t.Fatalf("expected no leading command on stderr for --json, got %q", stderr.String())
	}
	var resp tollbit.PagedSearchResultResponse
	if err := json.NewDecoder(&stdout).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Title != "Result" {
		t.Fatalf("unexpected json response: %#v", resp)
	}
}

func TestRunAuthSetStatusAndLogoutAll(t *testing.T) {
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"auth", "set", "--name", "agent-test", "--user-agent", "agent-test/0.1"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth set failed: code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "updated agent profile agent-test") {
		t.Fatalf("unexpected set stdout: %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "status"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth status failed: code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Agent:      agent-test") || !strings.Contains(stdout.String(), "User agent: agent-test/0.1") {
		t.Fatalf("unexpected status stdout: %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "logout", "--all"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth logout --all failed: code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Cleared agent profile and token.") {
		t.Fatalf("unexpected logout stdout: %q", stdout.String())
	}
}

func TestRunAuthStatusDefaultsToAnonymous(t *testing.T) {
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"auth", "status"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth status failed: code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Agent:      anonymous") {
		t.Fatalf("unexpected status stdout: %q", stdout.String())
	}
}

func TestRunAuthLoginStatusAndLogout(t *testing.T) {
	storageDir := t.TempDir()
	baseToken := testAgentJWT(t)
	oboToken := testAgentJWTWithOBO(t)
	var sawStart bool
	var sawRedeem bool

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.RequestURI() {
		case "/agent/v1/tokens/identity":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST token grant, got %s", r.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			switch body["grant_type"] {
			case "self_attested":
				_ = json.NewEncoder(w).Encode(map[string]string{"token": baseToken})
			case "consent:redirect":
				if body["agent_identifier"] != "agent-test" || body["code"] != "auth-code" || body["code_verifier"] == "" || body["redirect_uri"] == "" {
					t.Fatalf("unexpected redeem body: %#v", body)
				}
				sawRedeem = true
				refreshToken := "agrt_cli"
				refreshExpiresAt := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
				_ = json.NewEncoder(w).Encode(auth.AgentTokenResponse{Token: oboToken, RefreshToken: &refreshToken, RefreshTokenExpiresAt: &refreshExpiresAt})
			default:
				t.Fatalf("unexpected token grant body: %#v", body)
			}
		case "/agent/v1/consent/redirect/start":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST start, got %s", r.Method)
			}
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
				time.Sleep(50 * time.Millisecond)
				_, _ = http.Get(body.RedirectURI + "?code=auth-code&state=" + body.State)
			}()
			_ = json.NewEncoder(w).Encode(auth.ConsentRedirectStartResponse{
				ChallengeID: "ach_test",
				ConsentURL:  "https://hack.tollbit.test/oauth/consent-new?consent_challenge=ach_test",
				ExpiresAt:   time.Now().Add(time.Minute).Format(time.RFC3339),
			})
		case "/agent/v1/tokens/refresh/revoke":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST revoke, got %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(auth.RevokeRefreshTokenResponse{Revoked: true})
		default:
			t.Fatalf("unexpected auth request: %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer authSrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, storageDir)
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"auth", "login", "--name", "agent-test"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth login failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !sawStart || !sawRedeem {
		t.Fatalf("expected start and redeem requests, sawStart=%v sawRedeem=%v", sawStart, sawRedeem)
	}
	for _, want := range []string{"Authorize agent: agent-test", "Open this URL in your browser", "authorized as agent-test", "user usr_123", "org org_456"} {
		combined := stdout.String() + stderr.String()
		if !strings.Contains(combined, want) {
			t.Fatalf("expected login output to contain %q, got stdout=%q stderr=%q", want, stdout.String(), stderr.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "status"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth status failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"Agent:      agent-test", "Token:      valid", "On behalf:  user usr_123 / org org_456 (consent)"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected status stdout to contain %q, got %q", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "status", "--json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth json status failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var status map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode status json: %v\n%s", err, stdout.String())
	}
	identity := status["identity"].(map[string]any)
	if identity["name"] != "agent-test" {
		t.Fatalf("expected persisted identity agent-test, got %#v", identity)
	}
	tokenStatus := status["token"].(map[string]any)
	oboStatus := tokenStatus["obo"].(map[string]any)
	if oboStatus["source"] != "consent" || oboStatus["user"] != "usr_123" || oboStatus["org"] != "org_456" {
		t.Fatalf("unexpected obo status: %#v", oboStatus)
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "logout"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth logout failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Cleared agent token.") {
		t.Fatalf("unexpected logout stdout: %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "status"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth status after logout failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Token:      none") {
		t.Fatalf("expected token to be cleared, got %q", stdout.String())
	}
}

func TestRunAuthLogoutFailClosedAndForce(t *testing.T) {
	storageDir := t.TempDir()
	tokenPath := filepath.Join(storageDir, "agent-token.jwt")
	refreshPath := filepath.Join(storageDir, "refresh-token.json")
	if err := os.WriteFile(tokenPath, []byte(testAgentJWT(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	refreshRaw, err := json.Marshal(map[string]string{"refresh_token": "agrt_cli"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(refreshPath, refreshRaw, 0o600); err != nil {
		t.Fatal(err)
	}

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/agent/v1/tokens/refresh/revoke" {
			t.Fatalf("unexpected auth request: %s %s", r.Method, r.URL.RequestURI())
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"detail": "revoke failed"})
	}))
	defer authSrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, storageDir)

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"auth", "logout"}, nil, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit on revoke failure, got 0 stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "still logged in") {
		t.Fatalf("expected fail-closed message, got stderr=%q", stderr.String())
	}
	if _, err := os.Stat(tokenPath); err != nil {
		t.Fatalf("expected agent token to remain, got err=%v", err)
	}
	if _, err := os.Stat(refreshPath); err != nil {
		t.Fatalf("expected refresh token to remain, got err=%v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "logout", "--force"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth logout --force failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Cleared agent token.") {
		t.Fatalf("unexpected force logout stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: could not revoke the token on the server") {
		t.Fatalf("expected force warning on stderr, got %q", stderr.String())
	}
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Fatalf("expected agent token removed under force, got err=%v", err)
	}
	if _, err := os.Stat(refreshPath); !os.IsNotExist(err) {
		t.Fatalf("expected refresh token removed under force, got err=%v", err)
	}
}

func testAgentJWT(t *testing.T) string {
	t.Helper()
	claims := struct {
		jwt.RegisteredClaims
		TBT string `json:"tbt"`
	}{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "agent-test",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		TBT: "agent-token",
	}
	header := map[string]any{"alg": "none"}
	return encodeJSONSegment(t, header) + "." + encodeJSONSegment(t, claims) + "." + base64.RawURLEncoding.EncodeToString([]byte("signature"))
}

func testAgentJWTWithOBO(t *testing.T) string {
	t.Helper()
	claims := struct {
		jwt.RegisteredClaims
		TBT string         `json:"tbt"`
		OBO map[string]any `json:"obo"`
	}{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "agent-test",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		TBT: "agent-token",
		OBO: map[string]any{
			"ver": 1,
			"src": "consent",
			"usr": "usr_123",
			"org": "org_456",
		},
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

func TestRunAuthStatusCheckExitCodes(t *testing.T) {
	storageDir := t.TempDir()
	t.Setenv(testCredentialsStorageDirEnvVar, storageDir)

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"auth", "status", "--check"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 for missing token, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout with --check, got %q", stdout.String())
	}

	expiredToken := testAgentJWTExpired(t)
	if err := os.WriteFile(filepath.Join(storageDir, "agent-token.jwt"), []byte(expiredToken), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "status", "--check"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1 for invalid token, got %d stderr=%q", code, stderr.String())
	}

	validToken := testAgentJWT(t)
	if err := os.WriteFile(filepath.Join(storageDir, "agent-token.jwt"), []byte(validToken), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "status", "--check"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 for valid token, got %d stderr=%q", code, stderr.String())
	}
}

func TestRunAuthSetNameChangeClearsToken(t *testing.T) {
	storageDir := t.TempDir()
	t.Setenv(testCredentialsStorageDirEnvVar, storageDir)
	if err := os.WriteFile(filepath.Join(storageDir, "agent-token.jwt"), []byte(testAgentJWT(t)), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"auth", "set", "--name", "agent-test", "--user-agent", "agent-test/0.1"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth set failed: code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "set", "--name", "other-agent"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth set rename failed: code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "cleared token — profile updated") {
		t.Fatalf("expected token cleared notice, got stderr=%q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "status"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth status failed: code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Token:      none") {
		t.Fatalf("expected missing token after rename, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "User agent: agent-test/0.1") {
		t.Fatalf("expected user agent preserved, got %q", stdout.String())
	}
}

func testAgentJWTExpired(t *testing.T) string {
	t.Helper()
	claims := struct {
		jwt.RegisteredClaims
		TBT string `json:"tbt"`
	}{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "agent-test",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
		TBT: "agent-token",
	}
	header := map[string]any{"alg": "none"}
	return encodeJSONSegment(t, header) + "." + encodeJSONSegment(t, claims) + "." + base64.RawURLEncoding.EncodeToString([]byte("signature"))
}

func TestRunGuideInstallWritesSkillUnderSkillName(t *testing.T) {
	parentSkillsDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"guide", "--install", parentSkillsDir}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%q)", code, stderr.String())
	}
	wantPath := filepath.Join(parentSkillsDir, "tollbit-cli", "SKILL.md")
	absWant, err := filepath.Abs(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "installed SKILL.md at "+absWant) && !strings.Contains(stdout.String(), absWant) {
		t.Fatalf("stdout should include resolved path %q, got %q", absWant, stdout.String())
	}
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read installed skill at %s: %v", wantPath, err)
	}
	for _, want := range []string{"Configured remote flow: detached browser relay", "relay the printed URL and verification icon", "Auth instructions rendered for local=redirect, remote=browser_select_icon"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("expected installed skill to contain %q, got %q", want, string(got))
		}
	}
	if strings.Contains(string(got), "{{") {
		t.Fatalf("installed skill contains unrendered template syntax: %q", string(got))
	}
}

func TestRunGuideInstallDoesNotDoubleNestSkillDir(t *testing.T) {
	parent := t.TempDir()
	skillDir := filepath.Join(parent, "tollbit-cli")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"guide", "--install", skillDir}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	target := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected SKILL.md at %s: %v", target, err)
	}
	nested := filepath.Join(skillDir, "tollbit-cli")
	if fi, err := os.Stat(nested); err == nil && fi.IsDir() {
		t.Fatalf("unexpected nested directory %s (double-nest)", nested)
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"version"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != version.Version {
		t.Fatalf("stdout=%q want %q", stdout.String(), version.Version)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunHelpIncludesVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"help"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "version: "+version.Version) {
		t.Fatalf("help should include CLI version, got %q", stdout.String())
	}
}

func TestSkillFrontmatterVersionMatchesCLI(t *testing.T) {
	skillPath := filepath.Join("..", "..", "skill", "tollbit-cli", "SKILL.md")
	b, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	wantLine := "version: " + version.Version
	if !strings.Contains(string(b), wantLine) {
		t.Fatalf("skill should contain %q for sync with internal/version", wantLine)
	}
}

func TestRunGuideOutputsEmbeddedSkill(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"guide"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	for _, want := range []string{"Configured remote flow: detached browser relay", "relay the printed URL and verification icon", "`auth complete` takes: pending auth", "Auth instructions rendered for local=redirect, remote=browser_select_icon"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected guide output to contain %q, got %q", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "{{") {
		t.Fatalf("guide output contains unrendered template syntax: %q", stdout.String())
	}
}

func TestRunGuideRendersAgentConfirmsIconsCompleteInputs(t *testing.T) {
	config := testConfig()
	config.Auth.Consent.Strategy.Remote = configuration.ConsentStrategyAgentConfirmsIcons
	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"guide"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"Configured remote flow: detached icon confirmation", "relay only the printed consent URL", "`auth complete` takes: pending auth plus three icon names"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected guide output to contain %q, got %q", want, stdout.String())
		}
	}
}

func executeTestCommandWithConfig(config configuration.Config, args []string, stdin io.Reader, stdout, stderr *bytes.Buffer) int {
	cmd := NewCommandTree(app.Factory{Config: config})
	cmd.SetArgs(args)
	cmd.SetIn(stdin)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	err := cmd.Execute()
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
	return ExitCode(err)
}

func TestRuntimeSetAndStatus(t *testing.T) {
	storageDir := t.TempDir()
	runtimeDir := t.TempDir()
	config := testConfig()
	config.Credentials.StorageDir = storageDir
	config.Runtime.StateDir = runtimeDir

	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"runtime", "set", "--end-user-proximity", "remote"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runtime set failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "saved runtime end-user proximity remote") {
		t.Fatalf("unexpected runtime set stdout: %q", stdout.String())
	}
	info, err := os.Stat(filepath.Join(runtimeDir, "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected runtime state mode 0600, got %#o", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommandWithConfig(config, []string{"runtime", "status"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runtime status failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"Runtime state dir: " + runtimeDir,
		"Runtime state saved: true",
		"Credentials dir: " + storageDir,
		"End-user proximity: local (source: configured)",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected runtime status to contain %q, got %q", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommandWithConfig(config, []string{"--end-user-proximity", "auto-detect", "runtime", "status", "--json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runtime json status failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var status map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode status json: %v\n%s", err, stdout.String())
	}
	if status["configured_end_user_proximity"] != "auto-detect" || status["saved_end_user_proximity"] != "remote" || status["end_user_proximity"] != "remote" || status["end_user_proximity_source"] != "saved_runtime_state" || status["state_dir"] != runtimeDir || status["state_exists"] != true || status["credentials_dir"] != storageDir {
		t.Fatalf("unexpected runtime status: %#v", status)
	}
}

func TestRuntimeSetRejectsAutoDetect(t *testing.T) {
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())
	var stdout, stderr bytes.Buffer

	code := executeTestCommand([]string{"runtime", "set", "--end-user-proximity", "auto-detect"}, nil, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected usage error, got code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "runtime set --end-user-proximity must be local or remote") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRunAuthStatusShowsPendingAuthorization(t *testing.T) {
	storageDir := t.TempDir()
	t.Setenv(testCredentialsStorageDirEnvVar, storageDir)
	pending := agentauth.PendingConsent{
		Method:      agentauth.ConsentMethodBrowserSelectIcon,
		ChallengeID: "ach_test",
		AgentIdentity: auth.AgentIdentity{
			Name:      "pending-agent",
			UserAgent: "pending-agent/0.1",
		},
		BaseToken:    testAgentJWT(t),
		CodeVerifier: "verifier-1",
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
	}
	pendingJSON, err := json.Marshal(pending)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storageDir, "pending-auth.json"), pendingJSON, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"auth", "status"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth status failed: code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		"Agent:      anonymous",
		"Token:      none",
		"Auto-refresh: enabled",
		"Refresh:    absent",
		"Pending:    authorization pending (complete in browser, then run 'tollbit auth complete')",
		"Pending agent: pending-agent",
		"Pending user agent: pending-agent/0.1",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected status stdout to contain %q, got %q", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "browser_select_icon") {
		t.Fatalf("expected status stdout not to expose consent method, got %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "status", "--json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth json status failed: code=%d stderr=%q", code, stderr.String())
	}
	var status map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode status json: %v\n%s", err, stdout.String())
	}
	identity := status["identity"].(map[string]any)
	if identity["name"] != "anonymous" {
		t.Fatalf("expected active identity to remain anonymous, got %#v", identity)
	}
	pendingStatus := status["pending_authorization"].(map[string]any)
	if pendingStatus["pending"] != true || pendingStatus["challenge_id"] != "ach_test" {
		t.Fatalf("unexpected pending authorization status: %#v", pendingStatus)
	}
	if _, ok := pendingStatus["method"]; ok {
		t.Fatalf("expected pending authorization json not to expose method: %#v", pendingStatus)
	}
	pendingIdentity := pendingStatus["identity"].(map[string]any)
	if pendingIdentity["name"] != "pending-agent" || pendingIdentity["user_agent"] != "pending-agent/0.1" {
		t.Fatalf("unexpected pending identity status: %#v", pendingIdentity)
	}
	if status["auto_refresh"] != true {
		t.Fatalf("expected auto_refresh true, got %#v", status)
	}
	refreshStatus := status["refresh_token"].(map[string]any)
	if refreshStatus["present"] != false {
		t.Fatalf("unexpected refresh token status: %#v", refreshStatus)
	}

	stdout.Reset()
	stderr.Reset()
	code = executeTestCommand([]string{"auth", "status", "--check"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected check to fail until token exists, got code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunAuthLoginRemotePendingUsesRetryExitCode(t *testing.T) {
	storageDir := t.TempDir()
	baseToken := testAgentJWT(t)
	var sawStart bool

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/agent/v1/tokens/identity":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST token, got %s", r.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["grant_type"] != "self_attested" {
				t.Fatalf("unexpected token grant body: %#v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"token": baseToken})
		case "/agent/v1/consent/browser-select-icon/start":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST start, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Bearer "+baseToken {
				t.Fatalf("unexpected start authorization: %q", r.Header.Get("Authorization"))
			}
			var body auth.ConsentBrowserSelectIconStartRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.CodeChallenge == "" || body.CodeChallengeMethod != "S256" {
				t.Fatalf("unexpected start body: %#v", body)
			}
			sawStart = true
			_ = json.NewEncoder(w).Encode(auth.ConsentBrowserSelectIconStartResponse{
				ChallengeID: "ach_pending",
				ConsentURL:  "https://auth.example.test/oauth/consent/browser-select-icon?consent_challenge=ach_pending",
				ExpiresAt:   time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
				CorrectIcon: auth.AgentConsentIcon{Name: "SNAIL", Art: "@_"},
			})
		default:
			t.Fatalf("unexpected auth request: %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer authSrv.Close()

	config := testConfig()
	config.Auth.BaseURL = authSrv.URL
	config.Runtime.StateDir = storageDir
	config.Credentials.StorageDir = storageDir
	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"--end-user-proximity", "remote", "auth", "login", "--name", "agent-test"}, nil, &stdout, &stderr)
	if code != ExitCodeAuthorizationPending {
		t.Fatalf("expected pending exit code %d, got %d stdout=%q stderr=%q", ExitCodeAuthorizationPending, code, stdout.String(), stderr.String())
	}
	if !sawStart {
		t.Fatal("expected browser-select-icon start request")
	}
	for _, want := range []string{"Runtime end-user proximity: remote (configured)", "Authorization flow: detached browser relay", "Open this URL in the end user's browser", "BEGIN VERIFICATION ICON", "Authorization pending."} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected login stdout to contain %q, got %q", want, stdout.String())
		}
	}
	if _, err := os.Stat(filepath.Join(storageDir, "pending-auth.json")); err != nil {
		t.Fatalf("expected pending auth saved: %v", err)
	}
}

func TestRunAuthCompleteRejectedWithoutPendingAuth(t *testing.T) {
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())
	var stdout, stderr bytes.Buffer

	code := executeTestCommand([]string{"auth", "complete"}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "no pending authorization found") {
		t.Fatalf("expected no pending authorization error, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunAuthCompletePendingUsesRetryExitCode(t *testing.T) {
	storageDir := t.TempDir()
	baseToken := testAgentJWT(t)
	var sawRedeem bool
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/agent/v1/tokens/identity" {
			t.Fatalf("unexpected auth request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+baseToken {
			t.Fatalf("unexpected authorization: %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["grant_type"] == "consent" {
			t.Fatal("bare consent grant_type is not accepted")
		}
		if body["grant_type"] != "consent:browser_select_icon" || body["challenge_id"] != "ach_pending" || body["code_verifier"] != "verifier-1" {
			t.Fatalf("unexpected redeem body: %#v", body)
		}
		sawRedeem = true
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":   "https://errors.tollbit.com/authorization-pending",
			"title":  "Authorization pending",
			"status": http.StatusBadRequest,
			"code":   "authorization_pending",
		})
	}))
	defer authSrv.Close()
	config := testConfig()
	config.Auth.BaseURL = authSrv.URL
	config.Runtime.StateDir = storageDir
	config.Credentials.StorageDir = storageDir
	pending := agentauth.PendingConsent{
		Method:      agentauth.ConsentMethodBrowserSelectIcon,
		ChallengeID: "ach_pending",
		AgentIdentity: auth.AgentIdentity{
			Name: "pending-agent",
		},
		BaseToken:    baseToken,
		CodeVerifier: "verifier-1",
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
	}
	pendingJSON, err := json.Marshal(pending)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storageDir, "pending-auth.json"), pendingJSON, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := executeTestCommandWithConfig(config, []string{"--end-user-proximity", "remote", "auth", "complete"}, nil, &stdout, &stderr)

	if code != ExitCodeAuthorizationPending {
		t.Fatalf("expected pending exit code %d, got %d stdout=%q stderr=%q", ExitCodeAuthorizationPending, code, stdout.String(), stderr.String())
	}
	if !sawRedeem {
		t.Fatal("expected auth complete to check pending authorization")
	}
	if !strings.Contains(stderr.String(), "authorization still pending") {
		t.Fatalf("expected pending message, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(storageDir, "pending-auth.json")); err != nil {
		t.Fatalf("expected pending authorization to be preserved: %v", err)
	}
}

func writePendingConsentForTest(t *testing.T, storageDir string, pending agentauth.PendingConsent) {
	t.Helper()
	pendingJSON, err := json.Marshal(pending)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storageDir, "pending-auth.json"), pendingJSON, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRunAuthLoginRemoteAgentConfirmsIconsPendingUsesRetryExitCode(t *testing.T) {
	storageDir := t.TempDir()
	baseToken := testAgentJWT(t)
	var sawStart bool

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/agent/v1/tokens/identity":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": baseToken})
		case "/agent/v1/consent/agent-confirms-icons/start":
			if r.Header.Get("Authorization") != "Bearer "+baseToken {
				t.Fatalf("unexpected start authorization: %q", r.Header.Get("Authorization"))
			}
			var body auth.ConsentAgentConfirmsIconsStartRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.CodeChallenge == "" || body.CodeChallengeMethod != "S256" || body.Scope != "offline_access" {
				t.Fatalf("unexpected start body: %#v", body)
			}
			sawStart = true
			_ = json.NewEncoder(w).Encode(auth.ConsentAgentConfirmsIconsStartResponse{
				ChallengeID: "ach_pending",
				ConsentURL:  "https://auth.example.test/oauth/consent/agent-confirms-icons?consent_challenge=ach_pending",
				ExpiresAt:   time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
				IconNames:   []string{"ANCHOR", "FOX", "STAR"},
			})
		default:
			t.Fatalf("unexpected auth request: %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer authSrv.Close()

	config := testConfig()
	config.Auth.BaseURL = authSrv.URL
	config.Runtime.StateDir = storageDir
	config.Credentials.StorageDir = storageDir
	config.Auth.Consent.Strategy.Remote = configuration.ConsentStrategyAgentConfirmsIcons
	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"--end-user-proximity", "remote", "auth", "login", "--name", "agent-test"}, nil, &stdout, &stderr)
	if code != ExitCodeAuthorizationPending {
		t.Fatalf("expected pending exit code %d, got %d stdout=%q stderr=%q", ExitCodeAuthorizationPending, code, stdout.String(), stderr.String())
	}
	if !sawStart {
		t.Fatal("expected agent-confirms-icons start request")
	}
	for _, want := range []string{"Authorization flow: detached icon confirmation", "Relay ONLY this URL", "VALID ICON NAMES", "ANCHOR FOX STAR", "tollbit auth complete <first> <second> <third>"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected login stdout to contain %q, got %q", want, stdout.String())
		}
	}
	pendingRaw, err := os.ReadFile(filepath.Join(storageDir, "pending-auth.json"))
	if err != nil {
		t.Fatalf("expected pending auth saved: %v", err)
	}
	var pending agentauth.PendingConsent
	if err := json.Unmarshal(pendingRaw, &pending); err != nil {
		t.Fatal(err)
	}
	if len(pending.IconNames) != 3 || pending.IconNames[0] != "ANCHOR" || pending.IconNames[1] != "FOX" || pending.IconNames[2] != "STAR" {
		t.Fatalf("expected pending auth to contain icon names, got %#v", pending.IconNames)
	}
}

func TestRunAuthCompleteAgentConfirmsIconsSucceedsWithIconNames(t *testing.T) {
	storageDir := t.TempDir()
	baseToken := testAgentJWT(t)
	successToken := testAgentJWTWithOBO(t)
	var sawRedeem bool
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/agent/v1/tokens/identity" {
			t.Fatalf("unexpected auth request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		icons, ok := body["icon_names"].([]any)
		if !ok || len(icons) != 3 || icons[0] != "FOX" || icons[1] != "ANCHOR" || icons[2] != "STAR" {
			t.Fatalf("unexpected icon_names: %#v", body["icon_names"])
		}
		if body["grant_type"] != "consent:agent_confirms_icons" || body["challenge_id"] != "ach_pending" || body["code_verifier"] != "verifier-1" {
			t.Fatalf("unexpected redeem body: %#v", body)
		}
		sawRedeem = true
		_ = json.NewEncoder(w).Encode(map[string]string{"token": successToken})
	}))
	defer authSrv.Close()

	config := testConfig()
	config.Auth.BaseURL = authSrv.URL
	config.Runtime.StateDir = storageDir
	config.Credentials.StorageDir = storageDir
	config.Auth.Consent.Strategy.Remote = configuration.ConsentStrategyAgentConfirmsIcons
	writePendingConsentForTest(t, storageDir, agentauth.PendingConsent{
		Method:        agentauth.ConsentMethodAgentConfirmsIcons,
		ChallengeID:   "ach_pending",
		AgentIdentity: auth.AgentIdentity{Name: "pending-agent"},
		BaseToken:     baseToken,
		CodeVerifier:  "verifier-1",
		CreatedAt:     time.Now().UTC(),
		ExpiresAt:     time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
		IconNames:     []string{"ANCHOR", "FOX", "STAR"},
	})
	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"--end-user-proximity", "remote", "auth", "complete", "fox,", "Anchor", " STAR "}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected success, got code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !sawRedeem {
		t.Fatal("expected redeem request")
	}
	if !strings.Contains(stderr.String(), "authorized as pending-agent") {
		t.Fatalf("expected authorized message, got stderr=%q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(storageDir, "pending-auth.json")); !os.IsNotExist(err) {
		t.Fatalf("expected pending auth cleared, stat err=%v", err)
	}
}

func TestRunAuthCompleteAgentConfirmsIconsUnrecognizedIconKeepsPending(t *testing.T) {
	storageDir := t.TempDir()
	baseToken := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":   "https://errors.tollbit.com/unrecognized-icon",
			"title":  "Unrecognized icon",
			"status": http.StatusBadRequest,
			"detail": "unrecognized icon: FXX",
			"code":   "unrecognized_icon",
		})
	}))
	defer authSrv.Close()

	config := testConfig()
	config.Auth.BaseURL = authSrv.URL
	config.Runtime.StateDir = storageDir
	config.Credentials.StorageDir = storageDir
	config.Auth.Consent.Strategy.Remote = configuration.ConsentStrategyAgentConfirmsIcons
	writePendingConsentForTest(t, storageDir, agentauth.PendingConsent{
		Method:        agentauth.ConsentMethodAgentConfirmsIcons,
		ChallengeID:   "ach_pending",
		AgentIdentity: auth.AgentIdentity{Name: "pending-agent"},
		BaseToken:     baseToken,
		CodeVerifier:  "verifier-1",
		CreatedAt:     time.Now().UTC(),
		ExpiresAt:     time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
		IconNames:     []string{"ANCHOR", "FOX", "STAR"},
	})
	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"--end-user-proximity", "remote", "auth", "complete", "fxx", "anchor", "star"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "unrecognized icon: FXX") || !strings.Contains(stderr.String(), "Valid icon names: ANCHOR FOX STAR") {
		t.Fatalf("expected unrecognized icon guidance, got stderr=%q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(storageDir, "pending-auth.json")); err != nil {
		t.Fatalf("expected pending authorization to be preserved: %v", err)
	}
}

func TestRunAuthCompleteHelpRendersBothConfiguredFlows(t *testing.T) {
	tests := []struct {
		name           string
		remoteStrategy string
		want           []string
	}{
		{
			name:           "browser select icon",
			remoteStrategy: configuration.ConsentStrategyBrowserSelectIcon,
			want: []string{
				"Configured local completion flow (local browser):",
				"No detached completion command is used for the local browser flow",
				"Configured remote completion flow (detached browser relay):",
				"run `tollbit auth complete` with no arguments",
			},
		},
		{
			name:           "agent confirms icons",
			remoteStrategy: configuration.ConsentStrategyAgentConfirmsIcons,
			want: []string{
				"Configured local completion flow (local browser):",
				"No detached completion command is used for the local browser flow",
				"Configured remote completion flow (detached icon confirmation):",
				"tollbit auth complete <first> <second> <third>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := testConfig()
			config.Auth.Consent.Strategy.Remote = tt.remoteStrategy
			var stdout, stderr bytes.Buffer
			code := executeTestCommandWithConfig(config, []string{"auth", "complete", "--help"}, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
			}
			for _, want := range tt.want {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("expected help to contain %q, got %q", want, stdout.String())
				}
			}
		})
	}
}

func TestRunAuthHelpFallsBackOnBrokenConfig(t *testing.T) {
	config := testConfig()
	config.Auth.Consent.Strategy.Remote = "unknown"
	var stdout, stderr bytes.Buffer
	code := executeTestCommandWithConfig(config, []string{"auth", "complete", "--help"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Run `tollbit guide` for flow instructions.") {
		t.Fatalf("expected fallback guide pointer, got %q", stdout.String())
	}
}

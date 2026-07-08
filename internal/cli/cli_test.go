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
	"github.com/tollbit/tollbit-cli/internal/app"
	"github.com/tollbit/tollbit-cli/internal/client/auth"
	"github.com/tollbit/tollbit-cli/internal/client/tollbit"
	"github.com/tollbit/tollbit-cli/internal/configuration"
	"github.com/tollbit/tollbit-cli/internal/version"
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
		Auth: configuration.AuthConfig{
			BaseURL: authBaseURL,
			BrowserConsent: configuration.BrowserConsentConfig{
				CallbackAddress: "127.0.0.1:54321",
				Timeout:         3 * time.Minute,
				AutoOpenBrowser: false,
			},
		},
		Agent:       configuration.AgentConfig{DefaultName: agentDefaultName, DefaultUserAgent: agentDefaultUserAgent},
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
	if want := "page-2"; !strings.Contains(stdout.String(), want) {
		t.Fatalf("expected stdout to contain next-token hint, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
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
				t.Fatalf("expected POST mint, got %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"token": baseToken})
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
		case "/agent/v1/consent/redirect/token":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST redeem, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Bearer "+baseToken {
				t.Fatalf("unexpected redeem authorization: %q", r.Header.Get("Authorization"))
			}
			var body auth.ConsentRedirectTokenRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Code != "auth-code" || body.CodeVerifier == "" || body.RedirectURI == "" {
				t.Fatalf("unexpected redeem body: %#v", body)
			}
			sawRedeem = true
			_ = json.NewEncoder(w).Encode(map[string]string{"token": oboToken})
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
	skillPath := filepath.Join("..", "..", "skill", "tollbit-cli", "SKILL.md")
	want, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read source skill file: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("installed skill differs from embedded markdown")
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

	skillPath := filepath.Join("..", "..", "skill", "tollbit-cli", "SKILL.md")
	want, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read skill file: %v", err)
	}
	if stdout.String() != string(want) {
		t.Fatalf("guide output differs from embedded skill markdown")
	}
}

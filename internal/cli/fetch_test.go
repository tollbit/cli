package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tollbit/tollbit-cli/internal/client/tollbit"
	"github.com/tollbit/tollbit-cli/internal/errorsx/problemjson"
)

func TestRunFetchRendersBody(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := newFetchGatewayServer(t, token, fetchGatewayHandlers{
		rates: []tollbit.BatchRateResponseV2{{
			URL: "https://example.com/article",
			Rates: []tollbit.BatchDeveloperRateResponse{{
				Price:   tollbit.RatePriceResponse{PriceMicros: 50000, Currency: "USD"},
				License: tollbit.BatchRateLicenseResponse{LicenseType: "standard", Cuid: "lic_1"},
			}},
		}},
		contentToken: "content-jwt",
		contentBody:  "article body\n",
	})
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())

	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("y\n")
	code := executeTestCommand([]string{"content", "fetch", "https://example.com/article", "--user-agent", "MyAgent-User"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if stdout.String() != "article body\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Fetch will cost USD 0.05") {
		t.Fatalf("expected pricing prompt on stderr, got %q", stderr.String())
	}
}

func TestRunFetchConfirmSkipsPrompt(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := newFetchGatewayServer(t, token, fetchGatewayHandlers{
		rates: []tollbit.BatchRateResponseV2{{
			URL: "https://example.com/article",
			Rates: []tollbit.BatchDeveloperRateResponse{{
				Price:   tollbit.RatePriceResponse{PriceMicros: 50000, Currency: "USD"},
				License: tollbit.BatchRateLicenseResponse{LicenseType: "standard", Cuid: "lic_1"},
			}},
		}},
		contentToken: "content-jwt",
		contentBody:  "article body",
	})
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"content", "fetch", "https://example.com/article", "--confirm", "--user-agent", "MyAgent-User"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "Fetch will cost") {
		t.Fatalf("expected no pricing prompt, got stderr=%q", stderr.String())
	}
}

func TestRunFetchToDisk(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := newFetchGatewayServer(t, token, fetchGatewayHandlers{
		rates: []tollbit.BatchRateResponseV2{{
			URL: "https://example.com/article",
			Rates: []tollbit.BatchDeveloperRateResponse{{
				Price:   tollbit.RatePriceResponse{PriceMicros: 50000, Currency: "USD"},
				License: tollbit.BatchRateLicenseResponse{LicenseType: "standard", Cuid: "lic_1"},
			}},
		}},
		contentToken: "content-jwt",
		contentBody:  "saved body",
	})
	defer gatewaySrv.Close()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "article.md")

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{
		"content", "fetch", "https://example.com/article",
		"--confirm", "--toDisk", outPath,
		"--user-agent", "MyAgent-User",
	}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "saved body" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
}

func TestRunFetchJSON(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := newFetchGatewayServer(t, token, fetchGatewayHandlers{
		rates: []tollbit.BatchRateResponseV2{{
			URL: "https://example.com/article",
			Rates: []tollbit.BatchDeveloperRateResponse{{
				Price:   tollbit.RatePriceResponse{PriceMicros: 50000, Currency: "USD"},
				License: tollbit.BatchRateLicenseResponse{LicenseType: "standard", Cuid: "lic_1"},
			}},
		}},
		contentToken: "content-jwt",
		contentBody:  "json body",
	})
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{
		"content", "fetch", "https://example.com/article",
		"--confirm", "--json",
		"--user-agent", "MyAgent-User",
	}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	var resp tollbit.GetContentResponse
	if err := json.NewDecoder(&stdout).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Content.Body != "json body" {
		t.Fatalf("unexpected json response: %#v", resp)
	}
}

func TestRunFetchMultipleRatesRequiresIndexWithJSON(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := newFetchGatewayServer(t, token, fetchGatewayHandlers{
		rates: []tollbit.BatchRateResponseV2{{
			URL: "https://example.com/article",
			Rates: []tollbit.BatchDeveloperRateResponse{
				{Price: tollbit.RatePriceResponse{PriceMicros: 10000, Currency: "USD"}, License: tollbit.BatchRateLicenseResponse{LicenseType: "a", Cuid: "lic_1"}},
				{Price: tollbit.RatePriceResponse{PriceMicros: 20000, Currency: "USD"}, License: tollbit.BatchRateLicenseResponse{LicenseType: "b", Cuid: "lic_2"}},
			},
		}},
	})
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{
		"content", "fetch", "https://example.com/article",
		"--confirm", "--json",
		"--user-agent", "MyAgent-User",
	}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d stderr=%q", code, stderr.String())
	}
}

func TestRunFetchDeclineConfirm(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := newFetchGatewayServer(t, token, fetchGatewayHandlers{
		rates: []tollbit.BatchRateResponseV2{{
			URL: "https://example.com/article",
			Rates: []tollbit.BatchDeveloperRateResponse{{
				Price:   tollbit.RatePriceResponse{PriceMicros: 50000, Currency: "USD"},
				License: tollbit.BatchRateLicenseResponse{LicenseType: "standard", Cuid: "lic_1"},
			}},
		}},
	})
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{
		"content", "fetch", "https://example.com/article",
		"--user-agent", "MyAgent-User",
	}, strings.NewReader("n\n"), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d stderr=%q", code, stderr.String())
	}
}

func TestRunFetchUserAgentRegistrationRetry(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	storageDir := t.TempDir()
	tokenAttempts := 0
	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/agents/v1/rates/batch":
			_ = json.NewEncoder(w).Encode([]tollbit.BatchRateResponseV2{{
				URL: "https://example.com/article",
				Rates: []tollbit.BatchDeveloperRateResponse{{
					Price:   tollbit.RatePriceResponse{PriceMicros: 50000, Currency: "USD"},
					License: tollbit.BatchRateLicenseResponse{LicenseType: "standard", Cuid: "lic_1"},
				}},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/agents/v1/tokens/content":
			tokenAttempts++
			if tokenAttempts == 1 {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusBadRequest)
				code := problemjson.ErrorCodeUserAgentNotRegistered
				_ = json.NewEncoder(w).Encode(problemjson.Problem{
					Type: "about:blank", Title: "Bad Request", Status: 400, Code: &code,
				})
				return
			}
			_ = json.NewEncoder(w).Encode(tollbit.CreateContentAccessTokenResponse{Token: "content-jwt"})
		case r.Method == http.MethodGet && r.URL.Path == "/agents/v1/user-agents":
			_ = json.NewEncoder(w).Encode([]tollbit.UserAgentResponse{{
				Cuid: "ua_1", OrgCuid: "org_1", UserAgent: "Registered-Agent",
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/dev/v2/content/example.com/article":
			_ = json.NewEncoder(w).Encode(tollbit.GetContentResponse{Content: tollbit.PageContent{Body: "registered body"}})
		default:
			t.Fatalf("unexpected gateway request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, storageDir)

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{
		"content", "fetch", "https://example.com/article",
		"--confirm", "--user-agent", "Bad-Agent",
	}, strings.NewReader("1\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if stdout.String() != "registered body\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}

	identityRaw, err := os.ReadFile(filepath.Join(storageDir, "agent-identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(identityRaw), "Registered-Agent") {
		t.Fatalf("expected saved user agent, got %q", string(identityRaw))
	}
}

func TestRunFetchEmptyUserAgentListsAndRegisters(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	storageDir := t.TempDir()
	contentTokenCalled := false
	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/agents/v1/rates/batch":
			_ = json.NewEncoder(w).Encode([]tollbit.BatchRateResponseV2{{
				URL: "https://example.com/article",
				Rates: []tollbit.BatchDeveloperRateResponse{{
					Price:   tollbit.RatePriceResponse{PriceMicros: 50000, Currency: "USD"},
					License: tollbit.BatchRateLicenseResponse{LicenseType: "standard", Cuid: "lic_1"},
				}},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/agents/v1/user-agents":
			_ = json.NewEncoder(w).Encode([]tollbit.UserAgentResponse{{
				Cuid: "ua_1", OrgCuid: "org_1", UserAgent: "Registered-Agent",
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/agents/v1/tokens/content":
			contentTokenCalled = true
			_ = json.NewEncoder(w).Encode(tollbit.CreateContentAccessTokenResponse{Token: "content-jwt"})
		case r.Method == http.MethodGet && r.URL.Path == "/dev/v2/content/example.com/article":
			_ = json.NewEncoder(w).Encode(tollbit.GetContentResponse{Content: tollbit.PageContent{Body: "registered body"}})
		default:
			t.Fatalf("unexpected gateway request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, storageDir)

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{
		"content", "fetch", "https://example.com/article",
		"--confirm",
	}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if !contentTokenCalled {
		t.Fatal("expected content token request after registering user agent")
	}
	if stdout.String() != "registered body\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestRunFetchNoRegisteredUserAgents(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/agents/v1/rates/batch":
			_ = json.NewEncoder(w).Encode([]tollbit.BatchRateResponseV2{{
				URL: "https://example.com/article",
				Rates: []tollbit.BatchDeveloperRateResponse{{
					Price:   tollbit.RatePriceResponse{PriceMicros: 50000, Currency: "USD"},
					License: tollbit.BatchRateLicenseResponse{LicenseType: "standard", Cuid: "lic_1"},
				}},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/agents/v1/user-agents":
			_ = json.NewEncoder(w).Encode([]tollbit.UserAgentResponse{})
		default:
			t.Fatalf("unexpected gateway request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{
		"content", "fetch", "https://example.com/article",
		"--confirm",
	}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), createUserAgentURL) {
		t.Fatalf("expected create-user-agent URL in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "run this command again") {
		t.Fatalf("expected restart guidance in stderr, got %q", stderr.String())
	}
}

func TestRunFetchUsageError(t *testing.T) {
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())

	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"content", "fetch"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

type fetchGatewayHandlers struct {
	rates        []tollbit.BatchRateResponseV2
	contentToken string
	contentBody  string
}

func newFetchGatewayServer(t *testing.T, agentToken string, handlers fetchGatewayHandlers) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/agents/v1/rates/batch":
			if r.Header.Get("Authorization") != "Bearer "+agentToken {
				t.Fatalf("unexpected authorization: %q", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(handlers.rates)
		case r.Method == http.MethodPost && r.URL.Path == "/agents/v1/tokens/content":
			if r.Header.Get("Authorization") != "Bearer "+agentToken {
				t.Fatalf("unexpected authorization: %q", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(tollbit.CreateContentAccessTokenResponse{Token: handlers.contentToken})
		case r.Method == http.MethodGet && r.URL.Path == "/dev/v2/content/example.com/article":
			if r.Header.Get("TollbitToken") != handlers.contentToken {
				t.Fatalf("unexpected tollbit token: %q", r.Header.Get("TollbitToken"))
			}
			_ = json.NewEncoder(w).Encode(tollbit.GetContentResponse{
				Content: tollbit.PageContent{Body: handlers.contentBody},
			})
		default:
			t.Fatalf("unexpected gateway request: %s %s", r.Method, r.URL.Path)
		}
	}))
}

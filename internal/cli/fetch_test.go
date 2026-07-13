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

func TestRunFetchWithoutUserAgentUsesDefault(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

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
		case r.Method == http.MethodPost && r.URL.Path == "/agents/v1/tokens/content":
			contentTokenCalled = true
			var req map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if _, ok := req["userAgent"]; ok {
				t.Fatalf("expected userAgent to be omitted, got %#v", req["userAgent"])
			}
			_ = json.NewEncoder(w).Encode(tollbit.CreateContentAccessTokenResponse{Token: "content-jwt"})
		case r.Method == http.MethodGet && r.URL.Path == "/agents/v1/content/example.com/article":
			if r.Header.Get("Tollbit-User-Agent") != "" {
				t.Fatalf("expected empty Tollbit-User-Agent, got %q", r.Header.Get("Tollbit-User-Agent"))
			}
			_ = json.NewEncoder(w).Encode(tollbit.GetContentResponse{Content: tollbit.PageContent{Body: "default body"}})
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
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if !contentTokenCalled {
		t.Fatal("expected content token request")
	}
	if stdout.String() != "default body\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestRunFetchUserAgentNotRegisteredShowsError(t *testing.T) {
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
		case r.Method == http.MethodPost && r.URL.Path == "/agents/v1/tokens/content":
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusBadRequest)
			code := problemjson.ErrorCodeUserAgentNotRegistered
			detail := "user agent bad-agent is not registered"
			_ = json.NewEncoder(w).Encode(problemjson.Problem{
				Type: "about:blank", Title: "Bad Request", Status: 400, Code: &code, Detail: &detail,
			})
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
		"--confirm", "--user-agent", "bad-agent",
	}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "user agent bad-agent is not registered") {
		t.Fatalf("expected problem detail on stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), registerUserAgentURL) {
		t.Fatalf("expected registration URL on stderr, got %q", stderr.String())
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
		case r.Method == http.MethodGet && r.URL.Path == "/agents/v1/content/example.com/article":
			if r.Header.Get("Tollbit-Token") != handlers.contentToken {
				t.Fatalf("unexpected tollbit token: %q", r.Header.Get("Tollbit-Token"))
			}
			_ = json.NewEncoder(w).Encode(tollbit.GetContentResponse{
				Content: tollbit.PageContent{Body: handlers.contentBody},
			})
		default:
			t.Fatalf("unexpected gateway request: %s %s", r.Method, r.URL.Path)
		}
	}))
}

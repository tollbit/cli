package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tollbit/tollbit-cli/internal/client/tollbit"
)

func TestRunPricingRendersResults(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.RequestURI() != "/agent/v1/tokens/identity" {
			t.Fatalf("unexpected auth request: %s %s", r.Method, r.URL.RequestURI())
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/agents/v1/rates/batch" {
			t.Fatalf("unexpected gateway request: %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode([]tollbit.BatchRateResponseV2{{
			URL: "https://example.com/article",
			Rates: []tollbit.BatchDeveloperRateResponse{{
				Price:   tollbit.RatePriceResponse{PriceMicros: 50000, Currency: "USD"},
				License: tollbit.BatchRateLicenseResponse{LicenseType: "standard"},
			}},
		}})
	}))
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"pricing", "https://example.com/article", "--agent-name", "agent-test"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "https://example.com/article") {
		t.Fatalf("expected stdout to contain URL, got %q", out)
	}
	if !strings.Contains(out, "USD 0.05") {
		t.Fatalf("expected stdout to contain formatted price, got %q", out)
	}
	if !strings.Contains(out, "standard") {
		t.Fatalf("expected stdout to contain license type, got %q", out)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunContentPricingRendersResults(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/agents/v1/rates/batch" {
			t.Fatalf("unexpected gateway request: %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode([]tollbit.BatchRateResponseV2{{
			URL: "https://example.com/article",
			Rates: []tollbit.BatchDeveloperRateResponse{{
				Price:   tollbit.RatePriceResponse{PriceMicros: 50000, Currency: "USD"},
				License: tollbit.BatchRateLicenseResponse{LicenseType: "standard"},
			}},
		}})
	}))
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"content", "pricing", "https://example.com/article", "--agent-name", "agent-test"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "https://example.com/article") {
		t.Fatalf("expected stdout to contain URL, got %q", out)
	}
	if !strings.Contains(out, "USD 0.05") {
		t.Fatalf("expected stdout to contain formatted price, got %q", out)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunPricingJSON(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]tollbit.BatchRateResponseV2{{
			URL: "https://example.com/a",
			Rates: []tollbit.BatchDeveloperRateResponse{{
				Price:   tollbit.RatePriceResponse{PriceMicros: 100000, Currency: "USD"},
				License: tollbit.BatchRateLicenseResponse{LicenseType: "premium"},
			}},
		}})
	}))
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"pricing", "https://example.com/a", "--json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	var resp []tollbit.BatchRateResponseV2
	if err := json.NewDecoder(&stdout).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp) != 1 || resp[0].URL != "https://example.com/a" {
		t.Fatalf("unexpected json response: %#v", resp)
	}
}

func TestRunPricingUsageError(t *testing.T) {
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())

	t.Run("no args", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := executeTestCommand([]string{"pricing"}, nil, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("expected exit code 2, got %d stderr=%q", code, stderr.String())
		}
	})

	t.Run("invalid url", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := executeTestCommand([]string{"pricing", "not-a-url"}, nil, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("expected exit code 2, got %d stderr=%q", code, stderr.String())
		}
	})
}

func TestFormatPriceMicros(t *testing.T) {
	tests := []struct {
		micros   int64
		currency string
		want     string
	}{
		{50000, "USD", "USD 0.05"},
		{1000000, "USD", "USD 1"},
		{1500000, "EUR", "EUR 1.5"},
		{0, "USD", "USD 0"},
	}
	for _, tc := range tests {
		if got := formatPriceMicros(tc.micros, tc.currency); got != tc.want {
			t.Fatalf("formatPriceMicros(%d, %q) = %q, want %q", tc.micros, tc.currency, got, tc.want)
		}
	}
}

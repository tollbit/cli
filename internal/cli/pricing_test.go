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
	code := executeTestCommand([]string{"content", "pricing", "https://example.com/article"}, nil, &stdout, &stderr)

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
	if want := "To fetch content: tollbit content fetch https://example.com/article"; !strings.Contains(stderr.String(), want) {
		t.Fatalf("expected stderr to contain fetch leading command, got %q", stderr.String())
	}
	if strings.Contains(out, "To fetch content:") {
		t.Fatalf("expected fetch leading command on stderr, not stdout; got %q", out)
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
	code := executeTestCommand([]string{"content", "pricing", "https://example.com/article"}, nil, &stdout, &stderr)

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
	if want := "To fetch content: tollbit content fetch https://example.com/article"; !strings.Contains(stderr.String(), want) {
		t.Fatalf("expected stderr to contain fetch leading command, got %q", stderr.String())
	}
	if strings.Contains(out, "To fetch content:") {
		t.Fatalf("expected fetch leading command on stderr, not stdout; got %q", out)
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
	code := executeTestCommand([]string{"content", "pricing", "https://example.com/a", "--json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "To fetch content:") {
		t.Fatalf("expected no leading command in --json output, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "To fetch content:") {
		t.Fatalf("expected no leading command on stderr for --json, got %q", stderr.String())
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
		code := executeTestCommand([]string{"content", "pricing"}, nil, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("expected exit code 2, got %d stderr=%q", code, stderr.String())
		}
	})

	t.Run("invalid url", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := executeTestCommand([]string{"content", "pricing", "not-a-url"}, nil, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("expected exit code 2, got %d stderr=%q", code, stderr.String())
		}
	})
}

func TestNormalizeArticleURL(t *testing.T) {
	got, err := normalizeArticleURL("https://time.com/article/foo/")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://time.com/article/foo"
	if got != want {
		t.Fatalf("normalizeArticleURL() = %q, want %q", got, want)
	}

	got, err = normalizeArticleURL("https://time.com/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://time.com/" {
		t.Fatalf("normalizeArticleURL(root) = %q", got)
	}
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

func TestLicenseDisplayInfo(t *testing.T) {
	t.Run("on demand summarization", func(t *testing.T) {
		got := licenseDisplayInfo(tollbit.BatchRateLicenseResponse{LicenseType: licenseTypeOnDemand})
		if got.label != "Summarization" || got.description == "" || got.licenseURL != "" {
			t.Fatalf("unexpected display: %#v", got)
		}
	})

	t.Run("on demand full display", func(t *testing.T) {
		got := licenseDisplayInfo(tollbit.BatchRateLicenseResponse{LicenseType: licenseTypeOnDemandFullUse})
		if got.label != "Full Display" || got.description == "" || got.licenseURL != "" {
			t.Fatalf("unexpected display: %#v", got)
		}
	})

	t.Run("other license uses path", func(t *testing.T) {
		got := licenseDisplayInfo(tollbit.BatchRateLicenseResponse{
			LicenseType: "premium",
			LicensePath: "https://example.com/licenses/premium",
		})
		if got.label != "premium" || got.description != "" || got.licenseURL != "https://example.com/licenses/premium" {
			t.Fatalf("unexpected display: %#v", got)
		}
	})
}

func TestFormatPricingLicenseLabel(t *testing.T) {
	display := licenseDisplayInfo(tollbit.BatchRateLicenseResponse{LicenseType: licenseTypeOnDemand})
	got := formatPricingLicenseLabel(tollbit.BatchRateLicenseResponse{LicenseType: licenseTypeOnDemand}, display)
	if got != "Summarization (ON_DEMAND_LICENSE)" {
		t.Fatalf("got %q", got)
	}

	display = licenseDisplayInfo(tollbit.BatchRateLicenseResponse{
		LicenseType: "premium",
		LicensePath: "https://example.com/licenses/premium",
	})
	got = formatPricingLicenseLabel(tollbit.BatchRateLicenseResponse{LicenseType: "premium"}, display)
	if got != "premium" {
		t.Fatalf("got %q", got)
	}
}

func TestPrintPricingResultsLicenseDetails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	printPricingResults(&stdout, &stderr, []tollbit.BatchRateResponseV2{{
		URL: "https://example.com/article",
		Rates: []tollbit.BatchDeveloperRateResponse{
			{
				Price:   tollbit.RatePriceResponse{PriceMicros: 50000, Currency: "USD"},
				License: tollbit.BatchRateLicenseResponse{LicenseType: licenseTypeOnDemand},
			},
			{
				Price:   tollbit.RatePriceResponse{PriceMicros: 100000, Currency: "USD"},
				License: tollbit.BatchRateLicenseResponse{LicenseType: licenseTypeOnDemandFullUse},
			},
			{
				Price:   tollbit.RatePriceResponse{PriceMicros: 200000, Currency: "USD"},
				License: tollbit.BatchRateLicenseResponse{
					LicenseType: "premium",
					LicensePath: "https://example.com/licenses/premium",
				},
			},
		},
	}})

	out := stdout.String()
	if !strings.Contains(out, "Summarization (ON_DEMAND_LICENSE)") || !strings.Contains(out, "summaries and citations") {
		t.Fatalf("expected summarization detail, got %q", out)
	}
	if !strings.Contains(out, "Full Display (ON_DEMAND_FULL_USE_LICENSE)") || !strings.Contains(out, "full article text") {
		t.Fatalf("expected full display detail, got %q", out)
	}
	if !strings.Contains(out, "premium") || !strings.Contains(out, "https://example.com/licenses/premium") {
		t.Fatalf("expected premium license URL, got %q", out)
	}
	if strings.Contains(out, "To fetch content:") {
		t.Fatalf("expected fetch leading command on stderr, not stdout; got %q", out)
	}
	if want := "To fetch content: tollbit content fetch https://example.com/article"; !strings.Contains(stderr.String(), want) {
		t.Fatalf("expected fetch leading command on stderr, got %q", stderr.String())
	}
}

package configuration

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var testDefaultConfig = []byte("app:\n  name: tollbit\nauth:\n  base_url: https://oauth.tollbit.com\n  retry_on_obo_required: true\n  token_ttl_seconds: 0\n  use_refresh_tokens: true\n  browser_consent:\n    callback_address: 127.0.0.1:54321\n    timeout: 3m\n    auto_open_browser: true\nagent:\n  default_name: anonymous\n  default_user_agent: \"\"\ncredentials:\n  storage_dir: __default__\ngateway:\n  base_url: https://gateway.tollbit.com\n")

func TestAssembleConfigurationUsesEmbeddedDefaults(t *testing.T) {
	config := assembleTestConfiguration(t, t.TempDir())

	if config.App.Name != "tollbit" {
		t.Fatalf("expected app name tollbit, got %q", config.App.Name)
	}
	if config.Agent.DefaultName != "anonymous" {
		t.Fatalf("expected default agent name anonymous, got %q", config.Agent.DefaultName)
	}
	if config.Auth.BrowserConsent.Timeout != 3*time.Minute {
		t.Fatalf("expected browser consent timeout default, got %s", config.Auth.BrowserConsent.Timeout)
	}
	if !config.Auth.UseRefreshTokens {
		t.Fatal("expected refresh tokens enabled by default")
	}
	if config.Credentials.StorageDir == "" || config.Credentials.StorageDir == "__default__" {
		t.Fatalf("expected resolved credentials storage dir, got %q", config.Credentials.StorageDir)
	}
}

func TestAssembleConfigurationMergesDevelopmentConfig(t *testing.T) {
	dir := t.TempDir()
	writeDevelopmentConfig(t, dir, "gateway:\n  base_url: https://gateway-development.example.com\n")

	config := assembleTestConfiguration(t, dir)

	if config.Gateway.BaseURL != "https://gateway-development.example.com" {
		t.Fatalf("expected gateway development override, got %q", config.Gateway.BaseURL)
	}
}

func TestAssembleConfigurationEnvOverridesFiles(t *testing.T) {
	dir := t.TempDir()
	writeDevelopmentConfig(t, dir, "gateway:\n  base_url: https://gateway-development.example.com\n")
	t.Setenv("TOLLBIT_GATEWAY_BASE_URL", "https://gateway-env.example.com")

	config := assembleTestConfiguration(t, dir)

	if config.Gateway.BaseURL != "https://gateway-env.example.com" {
		t.Fatalf("expected gateway env override, got %q", config.Gateway.BaseURL)
	}
}

func TestAssembleConfigurationRequiresDefaultConfig(t *testing.T) {
	_, err := assembleConfiguration(nil, func() (string, error) { return t.TempDir(), nil })
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConfigWithOverridesAppliesAndValidates(t *testing.T) {
	config := assembleTestConfiguration(t, t.TempDir())
	gatewayBaseURL := "https://gateway-flag.example.com"
	timeout := 30 * time.Second
	autoOpenBrowser := false

	got, err := config.WithOverrides(OverrideOptions{
		GatewayBaseURL:                    &gatewayBaseURL,
		AuthBrowserConsentTimeout:         &timeout,
		AuthBrowserConsentAutoOpenBrowser: &autoOpenBrowser,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Gateway.BaseURL != gatewayBaseURL {
		t.Fatalf("expected gateway override, got %q", got.Gateway.BaseURL)
	}
	if got.Auth.BrowserConsent.Timeout != timeout {
		t.Fatalf("expected timeout override, got %s", got.Auth.BrowserConsent.Timeout)
	}
	if got.Auth.BrowserConsent.AutoOpenBrowser {
		t.Fatal("expected auto open browser override")
	}
	if config.Gateway.BaseURL == gatewayBaseURL {
		t.Fatal("expected original config to remain unchanged")
	}
}

func TestConfigWithOverridesRejectsInvalidConfig(t *testing.T) {
	config := assembleTestConfiguration(t, t.TempDir())
	blank := ""

	_, err := config.WithOverrides(OverrideOptions{AuthBaseURL: &blank})
	if err == nil {
		t.Fatal("expected error")
	}
}

func assembleTestConfiguration(t *testing.T, wd string) Config {
	t.Helper()
	config, err := assembleConfiguration(testDefaultConfig, func() (string, error) { return wd, nil })
	if err != nil {
		t.Fatal(err)
	}
	return config
}

func writeDevelopmentConfig(t *testing.T, dir string, content string) {
	t.Helper()
	path := filepath.Join(dir, developmentConfigFile)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

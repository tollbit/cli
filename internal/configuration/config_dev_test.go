//go:build dev

package configuration

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestAssembleConfigurationAppliesDevEndpointEnv(t *testing.T) {
	t.Setenv("TOLLBIT_AUTH_BASE_URL", "https://oauth-development.example")
	t.Setenv("TOLLBIT_GATEWAY_BASE_URL", "https://gateway-development.example.com")
	t.Setenv("TOLLBIT_AGENT_REGISTER_USER_AGENT_URL", "https://hack-development.example/my-agents")
	t.Setenv("TOLLBIT_AUTH_BROWSER_CONSENT_CALLBACK_ADDRESS", "127.0.0.1:65432")

	config := assembleTestConfiguration(t, t.TempDir())

	if config.Auth.BaseURL != "https://oauth-development.example" {
		t.Fatalf("expected auth env override, got %q", config.Auth.BaseURL)
	}
	if config.Gateway.BaseURL != "https://gateway-development.example.com" {
		t.Fatalf("expected gateway env override, got %q", config.Gateway.BaseURL)
	}
	if config.Agent.RegisterUserAgentURL != "https://hack-development.example/my-agents" {
		t.Fatalf("expected register URL env override, got %q", config.Agent.RegisterUserAgentURL)
	}
	if config.Auth.BrowserConsent.CallbackAddress != "127.0.0.1:65432" {
		t.Fatalf("expected callback env override, got %q", config.Auth.BrowserConsent.CallbackAddress)
	}
}

func TestAssembleConfigurationAppliesDevConsentStrategyOverrides(t *testing.T) {
	dir := t.TempDir()
	writeDevelopmentConfig(t, dir, "auth:\n  consent:\n    strategy:\n      local: browser_select_icon\n")

	config := assembleTestConfiguration(t, dir)
	if config.Auth.Consent.Strategy.Local != ConsentStrategyBrowserSelectIcon {
		t.Fatalf("expected development strategy override, got %q", config.Auth.Consent.Strategy.Local)
	}

	t.Setenv("TOLLBIT_AUTH_CONSENT_STRATEGY_LOCAL", ConsentStrategyRedirect)
	config = assembleTestConfiguration(t, dir)
	if config.Auth.Consent.Strategy.Local != ConsentStrategyRedirect {
		t.Fatalf("expected strategy env override to win, got %q", config.Auth.Consent.Strategy.Local)
	}
}

func TestIsConfigurableReportsDevEndpointFields(t *testing.T) {
	for _, path := range []string{"auth.base_url", "gateway.base_url", "agent.register_user_agent_url", "auth.browser_consent.callback_address", "auth.consent.strategy.local", "auth.consent.strategy.remote"} {
		if !IsConfigurable(path) {
			t.Fatalf("expected %s to be configurable in dev build", path)
		}
	}
}

func writeDevelopmentConfig(t *testing.T, dir string, content string) {
	t.Helper()
	path := filepath.Join(dir, developmentConfigFile)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

package configuration

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
	if config.Runtime.EndUserProximity != RuntimeEndUserProximityAutoDetect {
		t.Fatalf("expected auto-detect end-user proximity default, got %#v", config.Runtime)
	}
	if config.Auth.Consent.Strategy.Local != ConsentStrategyRedirect || config.Auth.Consent.Strategy.Remote != ConsentStrategyBrowserSelectIcon {
		t.Fatalf("expected default consent strategy mapping, got %#v", config.Auth.Consent)
	}
	if got := ResolveConsentStrategy(config); got != ConsentStrategyBrowserSelectIcon {
		t.Fatalf("expected auto-detect consent strategy %q, got %q", ConsentStrategyBrowserSelectIcon, got)
	}
	if config.Credentials.StorageDir == "" || config.Credentials.StorageDir == "__default__" {
		t.Fatalf("expected resolved credentials storage dir, got %q", config.Credentials.StorageDir)
	}
	if config.Runtime.StateDir == "" || config.Runtime.StateDir == "__default__" {
		t.Fatalf("expected resolved runtime state dir, got %q", config.Runtime.StateDir)
	}
	if config.Runtime.StateDir != config.Credentials.StorageDir {
		t.Fatalf("expected default runtime and credentials dirs to match, got runtime=%q credentials=%q", config.Runtime.StateDir, config.Credentials.StorageDir)
	}
}

func TestAssembleConfigurationRequiresDefaultConfig(t *testing.T) {
	_, err := assembleConfiguration(nil, func() (string, error) { return t.TempDir(), nil })
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAssembleConfigurationAppliesCommonEnv(t *testing.T) {
	t.Setenv("TOLLBIT_AGENT_DEFAULT_NAME", "env-agent")
	t.Setenv("TOLLBIT_AUTH_BROWSER_CONSENT_TIMEOUT", "30s")

	config := assembleTestConfiguration(t, t.TempDir())

	if config.Agent.DefaultName != "env-agent" {
		t.Fatalf("expected env agent name, got %q", config.Agent.DefaultName)
	}
	if config.Auth.BrowserConsent.Timeout != 30*time.Second {
		t.Fatalf("expected env browser timeout, got %s", config.Auth.BrowserConsent.Timeout)
	}
}

func TestIsConfigurableReportsCommonFields(t *testing.T) {
	if !IsConfigurable("agent.default_name") {
		t.Fatal("expected agent.default_name to be configurable")
	}
	if !IsConfigurable("runtime.end_user_proximity") || !IsConfigurable("runtime.state_dir") {
		t.Fatal("expected runtime fields to be configurable")
	}
	if IsConfigurable("auth.consent.strategy.local") != IsDev {
		t.Fatalf("expected auth.consent.strategy.local configurability to match IsDev=%v", IsDev)
	}
	if !IsConfigurable("auth.browser_consent.timeout") {
		t.Fatal("expected auth.browser_consent.timeout to be configurable")
	}
	if !IsConfigurable("credentials.storage_dir") {
		t.Fatal("expected credentials.storage_dir to be configurable")
	}
	if IsConfigurable("auth.base_url") != IsDev {
		t.Fatalf("expected auth.base_url configurability to match IsDev=%v", IsDev)
	}
	if IsConfigurable("app.name") {
		t.Fatal("expected untagged app.name to never be configurable")
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
	invalidEndUserProximity := "invalid"

	tests := []struct {
		name      string
		overrides OverrideOptions
	}{
		{
			name:      "blank auth base URL",
			overrides: OverrideOptions{AuthBaseURL: &blank},
		},
		{
			name:      "invalid end-user proximity value",
			overrides: OverrideOptions{RuntimeEndUserProximity: &invalidEndUserProximity},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.WithOverrides(tt.overrides)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestConfigWithOverridesAppliesRuntimeProximity(t *testing.T) {
	config := assembleTestConfiguration(t, t.TempDir())
	endUserProximity := RuntimeEndUserProximityLocal

	got, err := config.WithOverrides(OverrideOptions{RuntimeEndUserProximity: &endUserProximity})
	if err != nil {
		t.Fatal(err)
	}
	if got.Runtime.EndUserProximity != RuntimeEndUserProximityLocal {
		t.Fatalf("expected runtime end-user proximity override, got %#v", got.Runtime)
	}
	if strategy := ResolveConsentStrategy(got); strategy != ConsentStrategyRedirect {
		t.Fatalf("expected local consent strategy %q, got %q", ConsentStrategyRedirect, strategy)
	}
}

func assembleTestConfiguration(t *testing.T, wd string) Config {
	t.Helper()
	config, err := assembleConfiguration(readTestdata(t, "default-config.yaml"), func() (string, error) { return wd, nil })
	if err != nil {
		t.Fatal(err)
	}
	return config
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

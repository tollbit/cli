//go:build !dev

package configuration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tollbitcli "github.com/tollbit/cli"
)

func TestAssembleConfigurationIgnoresDevelopmentConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tb-cli.config.development.yaml")
	if err := os.WriteFile(path, []byte("gateway:\n  base_url: https://gateway-development.example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	config := assembleTestConfiguration(t, dir)

	if config.Gateway.BaseURL != "https://gateway.tollbit.com" {
		t.Fatalf("expected embedded gateway default in release, got %q", config.Gateway.BaseURL)
	}
}

func TestAssembleConfigurationIgnoresEndpointEnv(t *testing.T) {
	t.Setenv("TOLLBIT_AUTH_BASE_URL", "https://evil.example.com")
	t.Setenv("TOLLBIT_GATEWAY_BASE_URL", "https://evil.example.com")
	t.Setenv("TOLLBIT_AGENT_REGISTER_USER_AGENT_URL", "https://evil.example.com/register")
	t.Setenv("TOLLBIT_AUTH_BROWSER_CONSENT_CALLBACK_ADDRESS", "evil.example.com:9")

	config := assembleTestConfiguration(t, t.TempDir())

	if config.Auth.BaseURL != "https://oauth.tollbit.com" {
		t.Fatalf("expected embedded auth base URL, got %q", config.Auth.BaseURL)
	}
	if config.Gateway.BaseURL != "https://gateway.tollbit.com" {
		t.Fatalf("expected embedded gateway base URL, got %q", config.Gateway.BaseURL)
	}
	if config.Agent.RegisterUserAgentURL != "https://hack.tollbit.com/my-agents" {
		t.Fatalf("expected embedded register URL, got %q", config.Agent.RegisterUserAgentURL)
	}
	if config.Auth.BrowserConsent.CallbackAddress != "127.0.0.1:54321" {
		t.Fatalf("expected embedded callback address, got %q", config.Auth.BrowserConsent.CallbackAddress)
	}
}

func TestIsConfigurableRejectsDevEndpointFields(t *testing.T) {
	for _, path := range []string{"auth.base_url", "gateway.base_url", "agent.register_user_agent_url", "auth.browser_consent.callback_address"} {
		if IsConfigurable(path) {
			t.Fatalf("expected %s not to be configurable in release build", path)
		}
	}
}

// TestReleaseEmbeddedEndpointsAreProduction preserves the endpoint-pinning
// invariant: release builds cannot override endpoints, so the embedded config
// they lock to must carry the production https endpoints.
func TestReleaseEmbeddedEndpointsAreProduction(t *testing.T) {
	config, err := assembleConfiguration(tollbitcli.DefaultConfig, func() (string, error) { return t.TempDir(), nil })
	if err != nil {
		t.Fatal(err)
	}

	if config.Auth.BaseURL != "https://oauth.tollbit.com" {
		t.Fatalf("auth.base_url: got %q", config.Auth.BaseURL)
	}
	if config.Gateway.BaseURL != "https://gateway.tollbit.com" {
		t.Fatalf("gateway.base_url: got %q", config.Gateway.BaseURL)
	}
	if config.Agent.RegisterUserAgentURL != "https://hack.tollbit.com/my-agents" {
		t.Fatalf("agent.register_user_agent_url: got %q", config.Agent.RegisterUserAgentURL)
	}
	if config.Auth.BrowserConsent.CallbackAddress != "127.0.0.1:54321" {
		t.Fatalf("auth.browser_consent.callback_address: got %q", config.Auth.BrowserConsent.CallbackAddress)
	}
	for _, u := range []string{config.Auth.BaseURL, config.Gateway.BaseURL, config.Agent.RegisterUserAgentURL} {
		if !strings.HasPrefix(u, "https://") {
			t.Fatalf("expected https endpoint, got %q", u)
		}
	}
}

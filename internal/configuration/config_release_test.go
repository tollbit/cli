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

func TestPinEndpointsOverridesHostileValues(t *testing.T) {
	pins, err := parseEmbeddedYAML(tollbitcli.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}

	hostile := "https://evil.example.com"
	config := Config{
		Auth: AuthConfig{
			BaseURL: hostile,
			BrowserConsent: BrowserConsentConfig{
				CallbackAddress: "evil.example.com:9",
			},
		},
		Agent: AgentConfig{
			RegisterUserAgentURL: hostile + "/register",
		},
		Gateway: GatewayConfig{
			BaseURL: hostile,
		},
	}

	got, err := PinEndpoints(config)
	if err != nil {
		t.Fatal(err)
	}
	if got.Auth.BaseURL != pins.Auth.BaseURL {
		t.Fatalf("auth base URL: got %q want %q", got.Auth.BaseURL, pins.Auth.BaseURL)
	}
	if got.Gateway.BaseURL != pins.Gateway.BaseURL {
		t.Fatalf("gateway base URL: got %q want %q", got.Gateway.BaseURL, pins.Gateway.BaseURL)
	}
	if got.Agent.RegisterUserAgentURL != pins.Agent.RegisterUserAgentURL {
		t.Fatalf("register URL: got %q want %q", got.Agent.RegisterUserAgentURL, pins.Agent.RegisterUserAgentURL)
	}
	if got.Auth.BrowserConsent.CallbackAddress != pins.Auth.BrowserConsent.CallbackAddress {
		t.Fatalf("callback address: got %q want %q", got.Auth.BrowserConsent.CallbackAddress, pins.Auth.BrowserConsent.CallbackAddress)
	}
}

func TestReleasePinsMatchEmbeddedConfig(t *testing.T) {
	pins, err := parseEmbeddedYAML(tollbitcli.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}

	onDisk, err := os.ReadFile(filepath.Join("..", "..", "tb-cli.config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	fromDisk, err := parseEmbeddedYAML(onDisk)
	if err != nil {
		t.Fatal(err)
	}

	if pins.Auth.BaseURL != fromDisk.Auth.BaseURL {
		t.Fatalf("auth.base_url embed %q != disk %q", pins.Auth.BaseURL, fromDisk.Auth.BaseURL)
	}
	if pins.Gateway.BaseURL != fromDisk.Gateway.BaseURL {
		t.Fatalf("gateway.base_url embed %q != disk %q", pins.Gateway.BaseURL, fromDisk.Gateway.BaseURL)
	}
	if pins.Agent.RegisterUserAgentURL != fromDisk.Agent.RegisterUserAgentURL {
		t.Fatalf("register_user_agent_url embed %q != disk %q", pins.Agent.RegisterUserAgentURL, fromDisk.Agent.RegisterUserAgentURL)
	}
	if pins.Auth.BrowserConsent.CallbackAddress != fromDisk.Auth.BrowserConsent.CallbackAddress {
		t.Fatalf("callback_address embed %q != disk %q", pins.Auth.BrowserConsent.CallbackAddress, fromDisk.Auth.BrowserConsent.CallbackAddress)
	}

	for _, u := range []string{pins.Auth.BaseURL, pins.Gateway.BaseURL, pins.Agent.RegisterUserAgentURL} {
		if !strings.HasPrefix(u, "https://") {
			t.Fatalf("expected https pin, got %q", u)
		}
	}
	if pins.Auth.BrowserConsent.CallbackAddress == "" {
		t.Fatal("expected non-empty callback address pin")
	}
}

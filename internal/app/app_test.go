package app

import (
	"testing"

	"github.com/tollbit/cli/internal/configuration"
)

func TestNewExposesConfigAndBuildsClients(t *testing.T) {
	config := testConfig(t)
	app, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := app.Auth(); err != nil {
		t.Fatalf("expected auth client: %v", err)
	}
	if _, err := app.Tollbit(); err != nil {
		t.Fatalf("expected tollbit client: %v", err)
	}
	if _, err := app.OBOAuthorizer(); err != nil {
		t.Fatalf("expected OBO authorizer: %v", err)
	}
	if _, err := app.Credentials(); err != nil {
		t.Fatalf("expected credentials: %v", err)
	}
	if app.Config().App.Name != "test-cli" {
		t.Fatalf("expected app name test-cli, got %q", app.Config().App.Name)
	}
}

func testConfig(t *testing.T) configuration.Config {
	t.Helper()
	return configuration.Config{
		App: configuration.AppConfig{
			Name: "test-cli",
		},
		Auth: configuration.AuthConfig{
			BaseURL: "https://auth.example",
			BrowserConsent: configuration.BrowserConsentConfig{
				CallbackAddress: "127.0.0.1:54321",
				Timeout:         0,
				AutoOpenBrowser: false,
			},
		},
		Agent:       configuration.AgentConfig{DefaultName: "anonymous"},
		Credentials: configuration.CredentialsConfig{StorageDir: t.TempDir()},
		Gateway:     configuration.GatewayConfig{BaseURL: "https://gateway.example.com"},
	}
}

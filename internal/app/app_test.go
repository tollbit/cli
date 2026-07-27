package app

import (
	"testing"

	"github.com/tollbit/cli/internal/agentauth"
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

func TestNewBuildsBrowserSelectIconAuthorizer(t *testing.T) {
	config := testConfig(t)
	config.Runtime.EndUserProximity = configuration.RuntimeEndUserProximityRemote
	config.Auth.BrowserConsent.AutoOpenBrowser = true
	app, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	authorizer, err := app.OBOAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	if authorizer.SupportsOBORetry() {
		t.Fatal("browser-select-icon authorizer should not support OBO retry")
	}
	strategy, err := app.ConsentStrategy("browser_select_icon")
	if err != nil {
		t.Fatal(err)
	}
	if strategy.AutoOpensBrowser() {
		t.Fatal("remote browser-select-icon strategy should not auto-open browser")
	}
}

func TestConsentStrategyAutoOpenBrowserFollowsRuntimeAndConfig(t *testing.T) {
	tests := []struct {
		name                string
		endUserProximity    string
		autoOpenConfig      bool
		method              agentauth.ConsentMethod
		wantAutoOpenBrowser bool
	}{
		{
			name:                "local redirect enabled",
			endUserProximity:    configuration.RuntimeEndUserProximityLocal,
			autoOpenConfig:      true,
			method:              agentauth.ConsentMethodRedirect,
			wantAutoOpenBrowser: true,
		},
		{
			name:                "local redirect disabled by config",
			endUserProximity:    configuration.RuntimeEndUserProximityLocal,
			autoOpenConfig:      false,
			method:              agentauth.ConsentMethodRedirect,
			wantAutoOpenBrowser: false,
		},
		{
			name:                "remote browser select icon disabled by runtime",
			endUserProximity:    configuration.RuntimeEndUserProximityRemote,
			autoOpenConfig:      true,
			method:              agentauth.ConsentMethodBrowserSelectIcon,
			wantAutoOpenBrowser: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := testConfig(t)
			config.Runtime.EndUserProximity = tt.endUserProximity
			config.Auth.BrowserConsent.AutoOpenBrowser = tt.autoOpenConfig
			app, err := New(config)
			if err != nil {
				t.Fatal(err)
			}
			strategy, err := app.ConsentStrategy(tt.method)
			if err != nil {
				t.Fatal(err)
			}
			if strategy.AutoOpensBrowser() != tt.wantAutoOpenBrowser {
				t.Fatalf("expected AutoOpensBrowser=%v, got %v", tt.wantAutoOpenBrowser, strategy.AutoOpensBrowser())
			}
		})
	}
}

func testConfig(t *testing.T) configuration.Config {
	t.Helper()
	return configuration.Config{
		App: configuration.AppConfig{
			Name: "test-cli",
		},
		Runtime: configuration.RuntimeConfig{EndUserProximity: configuration.RuntimeEndUserProximityLocal, StateDir: t.TempDir()},
		Auth: configuration.AuthConfig{
			BaseURL: "https://auth.example",
			Consent: configuration.ConsentConfig{
				Strategy: configuration.ConsentStrategyConfig{
					Local:  configuration.ConsentStrategyRedirect,
					Remote: configuration.ConsentStrategyBrowserSelectIcon,
				},
			},
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

func TestBuildConsentStrategyAgentConfirmsIcons(t *testing.T) {
	config := testConfig(t)
	config.Runtime.EndUserProximity = configuration.RuntimeEndUserProximityRemote
	config.Auth.Consent.Strategy.Remote = configuration.ConsentStrategyAgentConfirmsIcons
	app, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	strategy, err := app.ConsentStrategy(agentauth.ConsentMethodAgentConfirmsIcons)
	if err != nil {
		t.Fatal(err)
	}
	if strategy.Method() != agentauth.ConsentMethodAgentConfirmsIcons {
		t.Fatalf("expected agent_confirms_icons method, got %q", strategy.Method())
	}
	if strategy.Guidance().FlowLabel != "detached icon confirmation" {
		t.Fatalf("unexpected guidance: %#v", strategy.Guidance())
	}
}

package globalflags

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/tollbit/cli/internal/configuration"
)

func TestAddRegistersOnlyConfigurableFlags(t *testing.T) {
	cmd := newTestRootCommand(t)
	for _, flag := range configFlags {
		if !configuration.IsConfigurable(flag.path) {
			if cmd.PersistentFlags().Lookup(flag.name) != nil {
				t.Fatalf("expected non-configurable flag %q to be unregistered", flag.name)
			}
			continue
		}
		if cmd.PersistentFlags().Lookup(flag.name) == nil {
			t.Fatalf("expected configurable flag %q to be registered", flag.name)
		}
	}
}

func TestOverridesFromCommandEmptyWithoutChanges(t *testing.T) {
	cmd := newTestRootCommand(t)
	overrides, err := OverridesFromCommand(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if overrides != (configuration.OverrideOptions{}) {
		t.Fatalf("expected empty overrides, got %#v", overrides)
	}
}

func newTestRootCommand(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "tollbit"}
	Add(cmd, configuration.Config{
		Auth: configuration.AuthConfig{
			BaseURL: "https://oauth.tollbit.com",
			BrowserConsent: configuration.BrowserConsentConfig{
				CallbackAddress: "127.0.0.1:54321",
				Timeout:         0,
				AutoOpenBrowser: true,
			},
		},
		Gateway:     configuration.GatewayConfig{BaseURL: "https://gateway.tollbit.com"},
		Credentials: configuration.CredentialsConfig{StorageDir: "/tmp/tollbit"},
	})
	return cmd
}

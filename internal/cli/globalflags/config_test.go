package globalflags

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tollbit/tollbit-cli/internal/configuration"
)

func TestDevFlagsHiddenByDefault(t *testing.T) {
	t.Setenv(ShowDevFlagsEnvVar, "")

	cmd := newTestRootCommand(t)
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Help(); err != nil {
		t.Fatalf("help: %v", err)
	}

	help := out.String()
	for _, name := range devFlagNames {
		if strings.Contains(help, name) {
			t.Fatalf("expected dev flag %q hidden from help, got:\n%s", name, help)
		}
	}
}

func TestDevFlagsVisibleWhenEnabled(t *testing.T) {
	t.Setenv(ShowDevFlagsEnvVar, "1")

	cmd := newTestRootCommand(t)
	for _, name := range devFlagNames {
		flag := cmd.PersistentFlags().Lookup(name)
		if flag == nil {
			t.Fatalf("missing dev flag %q", name)
		}
		if flag.Hidden {
			t.Fatalf("expected dev flag %q to be visible", name)
		}
	}
}

func TestDevFlagsStillAcceptedWhenHidden(t *testing.T) {
	t.Setenv(ShowDevFlagsEnvVar, "")

	cmd := newTestRootCommand(t)
	if err := cmd.ParseFlags([]string{"--gateway-base-url", "https://gateway.example.com"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if !cmd.PersistentFlags().Changed(FlagGatewayBaseURL) {
		t.Fatal("expected hidden dev flag to be parsed")
	}
}

func TestDevFlagsVisible(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "unset", value: "", want: false},
		{name: "zero", value: "0", want: false},
		{name: "false", value: "false", want: false},
		{name: "no", value: "no", want: false},
		{name: "off", value: "off", want: false},
		{name: "one", value: "1", want: true},
		{name: "true", value: "true", want: true},
		{name: "yes", value: "yes", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(ShowDevFlagsEnvVar, tt.value)
			if got := DevFlagsVisible(); got != tt.want {
				t.Fatalf("DevFlagsVisible() = %v, want %v", got, tt.want)
			}
		})
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

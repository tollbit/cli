//go:build !dev

package configuration

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
	tollbitcli "github.com/tollbit/cli"
)

var (
	releasePinsOnce sync.Once
	releasePins     Config
	releasePinsErr  error
)

// PinEndpoints forces security-sensitive endpoints to the values from the
// embedded tb-cli.config.yaml, discarding env/flag/dev-file overrides.
func PinEndpoints(c Config) (Config, error) {
	pins, err := loadReleasePins()
	if err != nil {
		return Config{}, err
	}
	c.Auth.BaseURL = pins.Auth.BaseURL
	c.Gateway.BaseURL = pins.Gateway.BaseURL
	c.Agent.RegisterUserAgentURL = pins.Agent.RegisterUserAgentURL
	c.Auth.BrowserConsent.CallbackAddress = pins.Auth.BrowserConsent.CallbackAddress
	return c, nil
}

// mergeDevConfig is a no-op in release builds — the development config file is
// never read, and the filename/merge code is compiled out entirely.
func mergeDevConfig(_ *viper.Viper, _ string) error { return nil }

func loadReleasePins() (Config, error) {
	releasePinsOnce.Do(func() {
		releasePins, releasePinsErr = parseEmbeddedYAML(tollbitcli.DefaultConfig)
		if releasePinsErr != nil {
			releasePinsErr = fmt.Errorf("parse release endpoint pins: %w", releasePinsErr)
		}
	})
	return releasePins, releasePinsErr
}

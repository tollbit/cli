//go:build dev

package configuration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const developmentConfigFile = "tb-cli.config.development.yaml"

// PinEndpoints is a no-op in dev builds; overrides are honored.
func PinEndpoints(c Config) (Config, error) { return c, nil }

// mergeDevConfig merges ./tb-cli.config.development.yaml from the working
// directory when present (developer convenience). Never compiled into release.
func mergeDevConfig(v *viper.Viper, wd string) error {
	return mergeConfigIfExists(v, filepath.Join(wd, developmentConfigFile))
}

func mergeConfigIfExists(v *viper.Viper, path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat development config: %w", err)
	}
	v.SetConfigFile(path)
	if err := v.MergeInConfig(); err != nil {
		return fmt.Errorf("merge development config: %w", err)
	}
	return nil
}

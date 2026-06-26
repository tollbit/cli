package configuration

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mapstructure "github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"github.com/tollbit/tollbit-cli/internal/configuration/platformdefaults"
)

const (
	developmentConfigFile = "tb-cli.config.development.yaml"
	envPrefix             = "TOLLBIT"
)

func AssembleConfiguration(defaultConfig []byte) (Config, error) {
	return assembleConfiguration(defaultConfig, os.Getwd)
}

func assembleConfiguration(defaultConfig []byte, getwd func() (string, error)) (Config, error) {
	if len(defaultConfig) == 0 {
		return Config{}, errors.New("default config is required")
	}

	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadConfig(bytes.NewReader(defaultConfig)); err != nil {
		return Config{}, fmt.Errorf("read embedded config: %w", err)
	}

	wd, err := getwd()
	if err != nil {
		return Config{}, fmt.Errorf("get working directory: %w", err)
	}
	if err := mergeConfigIfExists(v, filepath.Join(wd, developmentConfigFile)); err != nil {
		return Config{}, err
	}

	var config Config
	if err := v.Unmarshal(&config, viper.DecodeHook(mapstructure.StringToTimeDurationHookFunc())); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	config, err = resolveDefaults(config)
	if err != nil {
		return Config{}, err
	}
	if err := validate(config); err != nil {
		return Config{}, err
	}

	return config, nil
}

func resolveDefaults(config Config) (Config, error) {
	storageDir, err := platformdefaults.CredentialsStorageDir(config.Credentials.StorageDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve credentials.storage_dir: %w", err)
	}
	config.Credentials.StorageDir = storageDir
	return config, nil
}

func validate(config Config) error {
	if strings.TrimSpace(config.App.Name) == "" {
		return errors.New("app.name is required")
	}
	if strings.TrimSpace(config.Auth.BaseURL) == "" {
		return errors.New("auth.base_url is required")
	}
	if strings.TrimSpace(config.Agent.DefaultName) == "" {
		return errors.New("agent.default_name is required")
	}
	if strings.TrimSpace(config.Gateway.BaseURL) == "" {
		return errors.New("gateway.base_url is required")
	}
	if strings.TrimSpace(config.Auth.BrowserConsent.CallbackAddress) == "" {
		return errors.New("auth.browser_consent.callback_address is required")
	}
	if config.Auth.BrowserConsent.Timeout < 0 {
		return errors.New("auth.browser_consent.timeout must be non-negative")
	}
	if config.Auth.TokenTTLSeconds < 0 {
		return errors.New("auth.token_ttl_seconds must be non-negative")
	}
	if strings.TrimSpace(config.Credentials.StorageDir) == "" {
		return errors.New("credentials.storage_dir is required")
	}
	return nil
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

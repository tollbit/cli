package configuration

import "time"

type Config struct {
	App         AppConfig
	Auth        AuthConfig
	Agent       AgentConfig
	Credentials CredentialsConfig
	Gateway     GatewayConfig
}

type AppConfig struct {
	Name string
}

type AuthConfig struct {
	BaseURL            string               `mapstructure:"base_url" configurable:"dev"`
	RetryOnOBORequired bool                 `mapstructure:"retry_on_obo_required" configurable:"release"`
	TokenTTLSeconds    int32                `mapstructure:"token_ttl_seconds" configurable:"release"`
	UseRefreshTokens   bool                 `mapstructure:"use_refresh_tokens" configurable:"release"`
	BrowserConsent     BrowserConsentConfig `mapstructure:"browser_consent"`
}

type AgentConfig struct {
	DefaultName          string `mapstructure:"default_name" configurable:"release"`
	DefaultUserAgent     string `mapstructure:"default_user_agent" configurable:"release"`
	RegisterUserAgentURL string `mapstructure:"register_user_agent_url" configurable:"dev"`
}

type CredentialsConfig struct {
	StorageDir string `mapstructure:"storage_dir" configurable:"release"`
}

type GatewayConfig struct {
	BaseURL string `mapstructure:"base_url" configurable:"dev"`
}

type BrowserConsentConfig struct {
	CallbackAddress string        `mapstructure:"callback_address" configurable:"dev"`
	Timeout         time.Duration `mapstructure:"timeout" configurable:"release"`
	AutoOpenBrowser bool          `mapstructure:"auto_open_browser" configurable:"release"`
}

type OverrideOptions struct {
	AuthBaseURL                       *string
	AuthRetryOnOBORequired            *bool
	AuthTokenTTLSeconds               *int32
	AuthUseRefreshTokens              *bool
	AuthBrowserConsentCallbackAddress *string
	AuthBrowserConsentTimeout         *time.Duration
	AuthBrowserConsentAutoOpenBrowser *bool
	CredentialsStorageDir             *string
	GatewayBaseURL                    *string
}

func (c Config) WithOverrides(opts OverrideOptions) (Config, error) {
	if opts.AuthBaseURL != nil {
		c.Auth.BaseURL = *opts.AuthBaseURL
	}
	if opts.AuthRetryOnOBORequired != nil {
		c.Auth.RetryOnOBORequired = *opts.AuthRetryOnOBORequired
	}
	if opts.AuthTokenTTLSeconds != nil {
		c.Auth.TokenTTLSeconds = *opts.AuthTokenTTLSeconds
	}
	if opts.AuthUseRefreshTokens != nil {
		c.Auth.UseRefreshTokens = *opts.AuthUseRefreshTokens
	}
	if opts.AuthBrowserConsentCallbackAddress != nil {
		c.Auth.BrowserConsent.CallbackAddress = *opts.AuthBrowserConsentCallbackAddress
	}
	if opts.AuthBrowserConsentTimeout != nil {
		c.Auth.BrowserConsent.Timeout = *opts.AuthBrowserConsentTimeout
	}
	if opts.AuthBrowserConsentAutoOpenBrowser != nil {
		c.Auth.BrowserConsent.AutoOpenBrowser = *opts.AuthBrowserConsentAutoOpenBrowser
	}
	if opts.CredentialsStorageDir != nil {
		c.Credentials.StorageDir = *opts.CredentialsStorageDir
	}
	if opts.GatewayBaseURL != nil {
		c.Gateway.BaseURL = *opts.GatewayBaseURL
	}
	var err error
	c, err = resolveDefaults(c)
	if err != nil {
		return Config{}, err
	}
	if err := validate(c); err != nil {
		return Config{}, err
	}
	return c, nil
}

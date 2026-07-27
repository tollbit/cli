package globalflags

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tollbit/cli/internal/configuration"
)

type (
	configFlagOptions struct {
		authBaseURL                       string
		gatewayBaseURL                    string
		authRetryOnOBORequired            bool
		authTokenTTLSeconds               int32
		authUseRefreshTokens              bool
		authBrowserConsentCallbackAddress string
		authBrowserConsentTimeout         time.Duration
		authBrowserConsentAutoOpenBrowser bool
		credentialsStorageDir             string
	}

	configFlag struct {
		name        string
		path        string
		usage       string
		takesValue  bool
		hideDefault bool
		add         func(*pflag.FlagSet, *configFlagOptions, configFlag)
		apply       func(*pflag.FlagSet, *configuration.OverrideOptions, configFlag) error
	}
)

var (
	configFlags = []configFlag{
		// Removing from active flags for now
		// boolFlag(
		// 	"auth-retry-on-obo-required",
		// 	"auth.retry_on_obo_required",
		// 	"retry once with OBO authorization when required",
		// 	func(opts *configFlagOptions) bool { return opts.authRetryOnOBORequired },
		// 	func(overrides *configuration.OverrideOptions, value *bool) {
		// 		overrides.AuthRetryOnOBORequired = value
		// 	},
		// ),
		// Removing from active flags for now
		// int32Flag(
		// 	"auth-token-ttl-seconds",
		// 	"auth.token_ttl_seconds",
		// 	"agent token TTL in seconds; 0 uses server default",
		// 	func(opts *configFlagOptions) int32 { return opts.authTokenTTLSeconds },
		// 	func(overrides *configuration.OverrideOptions, value *int32) {
		// 		overrides.AuthTokenTTLSeconds = value
		// 	},
		// ),
		// Removing from active flags for now
		// boolFlag(
		// 	"auth-use-refresh-tokens",
		// 	"auth.use_refresh_tokens",
		// 	"request and use refresh tokens for OBO authorization",
		// 	func(opts *configFlagOptions) bool { return opts.authUseRefreshTokens },
		// 	func(overrides *configuration.OverrideOptions, value *bool) {
		// 		overrides.AuthUseRefreshTokens = value
		// 	},
		// ),
		// Removing from active flags for now
		// durationFlag(
		// 	"auth-browser-consent-timeout",
		// 	"auth.browser_consent.timeout",
		// 	"auth browser consent timeout",
		// 	func(opts *configFlagOptions) time.Duration { return opts.authBrowserConsentTimeout },
		// 	func(overrides *configuration.OverrideOptions, value *time.Duration) {
		// 		overrides.AuthBrowserConsentTimeout = value
		// 	},
		// ),
		// Removing from active flags for now
		// boolFlag(
		// 	"auth-browser-consent-auto-open-browser",
		// 	"auth.browser_consent.auto_open_browser",
		// 	"automatically open browser for auth consent",
		// 	func(opts *configFlagOptions) bool { return opts.authBrowserConsentAutoOpenBrowser },
		// 	func(overrides *configuration.OverrideOptions, value *bool) {
		// 		overrides.AuthBrowserConsentAutoOpenBrowser = value
		// 	},
		// ),
		// Removing from active flags for now
		// stringFlag(
		// 	"credentials-storage-dir",
		// 	"credentials.storage_dir",
		// 	"credential storage directory; __default__ uses the platform default",
		// 	func(opts *configFlagOptions) string { return opts.credentialsStorageDir },
		// 	func(overrides *configuration.OverrideOptions, value *string) {
		// 		overrides.CredentialsStorageDir = value
		// 	},
		// 	false,
		// ),
		// Removing from active flags for now
		// stringFlag(
		// 	"auth-base-url",
		// 	"auth.base_url",
		// 	"Auth API base URL",
		// 	func(opts *configFlagOptions) string { return opts.authBaseURL },
		// 	func(overrides *configuration.OverrideOptions, value *string) {
		// 		overrides.AuthBaseURL = value
		// 	},
		// 	false,
		// ),
		// Removing from active flags for now
		// stringFlag(
		// 	"gateway-base-url",
		// 	"gateway.base_url",
		// 	"Gateway API base URL",
		// 	func(opts *configFlagOptions) string { return opts.gatewayBaseURL },
		// 	func(overrides *configuration.OverrideOptions, value *string) {
		// 		overrides.GatewayBaseURL = value
		// 	},
		// 	false,
		// ),
		// Removing from active flags for now
		// stringFlag(
		// 	"auth-browser-consent-callback-address",
		// 	"auth.browser_consent.callback_address",
		// 	"auth browser consent callback address",
		// 	func(opts *configFlagOptions) string {
		// 		return opts.authBrowserConsentCallbackAddress
		// 	},
		// 	func(overrides *configuration.OverrideOptions, value *string) {
		// 		overrides.AuthBrowserConsentCallbackAddress = value
		// 	},
		// 	false,
		// ),
	}
)

func newConfigFlagOptions(config configuration.Config) *configFlagOptions {
	return &configFlagOptions{
		authBaseURL:                       config.Auth.BaseURL,
		gatewayBaseURL:                    config.Gateway.BaseURL,
		authRetryOnOBORequired:            config.Auth.RetryOnOBORequired,
		authTokenTTLSeconds:               config.Auth.TokenTTLSeconds,
		authUseRefreshTokens:              config.Auth.UseRefreshTokens,
		authBrowserConsentCallbackAddress: config.Auth.BrowserConsent.CallbackAddress,
		authBrowserConsentTimeout:         config.Auth.BrowserConsent.Timeout,
		authBrowserConsentAutoOpenBrowser: config.Auth.BrowserConsent.AutoOpenBrowser,
		credentialsStorageDir:             config.Credentials.StorageDir,
	}
}

func stringFlag(name string, path string, usage string, defaultValue func(*configFlagOptions) string, setOverride func(*configuration.OverrideOptions, *string), hideDefault bool) configFlag {
	return configFlag{
		name:        name,
		path:        path,
		usage:       usage,
		takesValue:  true,
		hideDefault: hideDefault,
		add: func(flags *pflag.FlagSet, opts *configFlagOptions, flag configFlag) {
			value := defaultValue(opts)
			flags.StringVar(&value, flag.name, value, flag.usage)
		},
		apply: func(flags *pflag.FlagSet, overrides *configuration.OverrideOptions, flag configFlag) error {
			value, err := changedStr(flags, flag.name)
			if err != nil {
				return err
			}
			setOverride(overrides, value)
			return nil
		},
	}
}

func boolFlag(name string, path string, usage string, defaultValue func(*configFlagOptions) bool, setOverride func(*configuration.OverrideOptions, *bool)) configFlag {
	return configFlag{
		name:       name,
		path:       path,
		usage:      usage,
		takesValue: false,
		add: func(flags *pflag.FlagSet, opts *configFlagOptions, flag configFlag) {
			value := defaultValue(opts)
			flags.BoolVar(&value, flag.name, value, flag.usage)
		},
		apply: func(flags *pflag.FlagSet, overrides *configuration.OverrideOptions, flag configFlag) error {
			if !flags.Changed(flag.name) {
				setOverride(overrides, nil)
				return nil
			}
			value, err := flags.GetBool(flag.name)
			if err != nil {
				return fmt.Errorf("read %s: %w", flag.name, err)
			}
			setOverride(overrides, &value)
			return nil
		},
	}
}

func int32Flag(name string, path string, usage string, defaultValue func(*configFlagOptions) int32, setOverride func(*configuration.OverrideOptions, *int32)) configFlag {
	return configFlag{
		name:       name,
		path:       path,
		usage:      usage,
		takesValue: true,
		add: func(flags *pflag.FlagSet, opts *configFlagOptions, flag configFlag) {
			value := defaultValue(opts)
			flags.Int32Var(&value, flag.name, value, flag.usage)
		},
		apply: func(flags *pflag.FlagSet, overrides *configuration.OverrideOptions, flag configFlag) error {
			if !flags.Changed(flag.name) {
				setOverride(overrides, nil)
				return nil
			}
			value, err := flags.GetInt32(flag.name)
			if err != nil {
				return fmt.Errorf("read %s: %w", flag.name, err)
			}
			setOverride(overrides, &value)
			return nil
		},
	}
}

func durationFlag(name string, path string, usage string, defaultValue func(*configFlagOptions) time.Duration, setOverride func(*configuration.OverrideOptions, *time.Duration)) configFlag {
	return configFlag{
		name:       name,
		path:       path,
		usage:      usage,
		takesValue: true,
		add: func(flags *pflag.FlagSet, opts *configFlagOptions, flag configFlag) {
			value := defaultValue(opts)
			flags.DurationVar(&value, flag.name, value, flag.usage)
		},
		apply: func(flags *pflag.FlagSet, overrides *configuration.OverrideOptions, flag configFlag) error {
			if !flags.Changed(flag.name) {
				setOverride(overrides, nil)
				return nil
			}
			value, err := flags.GetDuration(flag.name)
			if err != nil {
				return fmt.Errorf("read %s: %w", flag.name, err)
			}
			setOverride(overrides, &value)
			return nil
		},
	}
}

func OverridesFromCommand(cmd *cobra.Command) (configuration.OverrideOptions, error) {
	var overrides configuration.OverrideOptions
	flags := cmd.Root().PersistentFlags()
	for _, flag := range configFlags {
		if !configuration.IsConfigurable(flag.path) {
			continue
		}
		if err := flag.apply(flags, &overrides, flag); err != nil {
			return overrides, err
		}
	}
	return overrides, nil
}

func Add(cmd *cobra.Command, config configuration.Config) {
	opts := newConfigFlagOptions(config)
	flags := cmd.PersistentFlags()
	for _, flag := range configFlags {
		if !configuration.IsConfigurable(flag.path) {
			continue
		}
		flag.add(flags, opts, flag)
		if flag.hideDefault {
			flags.Lookup(flag.name).DefValue = ""
		}
	}
}

func changedStr(flags *pflag.FlagSet, name string) (*string, error) {
	if !flags.Changed(name) {
		return nil, nil
	}
	value, err := flags.GetString(name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	value = strings.TrimSpace(value)
	return &value, nil
}

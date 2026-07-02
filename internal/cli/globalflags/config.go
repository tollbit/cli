package globalflags

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tollbit/tollbit-cli/internal/configuration"
)

const ShowDevFlagsEnvVar = "TOLLBIT_SHOW_DEV_FLAGS"

const (
	FlagAuthBaseURL                       = "auth-base-url"
	FlagAuthRetryOnOBORequired            = "auth-retry-on-obo-required"
	FlagAuthTokenTTLSeconds               = "auth-token-ttl-seconds"
	FlagAuthBrowserConsentCallbackAddress = "auth-browser-consent-callback-address"
	FlagAuthBrowserConsentTimeout         = "auth-browser-consent-timeout"
	FlagAuthBrowserConsentAutoOpenBrowser = "auth-browser-consent-auto-open-browser"
	FlagCredentialsStorageDir             = "credentials-storage-dir"
	FlagGatewayBaseURL                    = "gateway-base-url"
)

var devFlagNames = []string{
	FlagAuthBaseURL,
	FlagGatewayBaseURL,
	FlagAuthRetryOnOBORequired,
	FlagAuthTokenTTLSeconds,
	FlagAuthBrowserConsentCallbackAddress,
	FlagAuthBrowserConsentTimeout,
	FlagAuthBrowserConsentAutoOpenBrowser,
	FlagCredentialsStorageDir,
}

type configFlagOptions struct {
	authBaseURL                       string
	gatewayBaseURL                    string
	authRetryOnOBORequired            bool
	authTokenTTLSeconds               int32
	authBrowserConsentCallbackAddress string
	authBrowserConsentTimeout         time.Duration
	authBrowserConsentAutoOpenBrowser bool
	credentialsStorageDir             string
}

type flagValue interface {
	~bool | ~int32 | time.Duration
}

func Add(cmd *cobra.Command, config configuration.Config) {
	opts := &configFlagOptions{
		authBaseURL:                       config.Auth.BaseURL,
		gatewayBaseURL:                    config.Gateway.BaseURL,
		authRetryOnOBORequired:            config.Auth.RetryOnOBORequired,
		authTokenTTLSeconds:               config.Auth.TokenTTLSeconds,
		authBrowserConsentCallbackAddress: config.Auth.BrowserConsent.CallbackAddress,
		authBrowserConsentTimeout:         config.Auth.BrowserConsent.Timeout,
		authBrowserConsentAutoOpenBrowser: config.Auth.BrowserConsent.AutoOpenBrowser,
		credentialsStorageDir:             config.Credentials.StorageDir,
	}
	flags := cmd.PersistentFlags()
	flags.StringVar(&opts.authBaseURL, FlagAuthBaseURL, opts.authBaseURL, "Auth API base URL")
	flags.StringVar(&opts.gatewayBaseURL, FlagGatewayBaseURL, opts.gatewayBaseURL, "Gateway API base URL")
	flags.BoolVar(&opts.authRetryOnOBORequired, FlagAuthRetryOnOBORequired, opts.authRetryOnOBORequired, "retry once with OBO authorization when required")
	flags.Int32Var(&opts.authTokenTTLSeconds, FlagAuthTokenTTLSeconds, opts.authTokenTTLSeconds, "agent token TTL in seconds; 0 uses server default")
	flags.StringVar(&opts.authBrowserConsentCallbackAddress, FlagAuthBrowserConsentCallbackAddress, opts.authBrowserConsentCallbackAddress, "auth browser consent callback address")
	flags.DurationVar(&opts.authBrowserConsentTimeout, FlagAuthBrowserConsentTimeout, opts.authBrowserConsentTimeout, "auth browser consent timeout")
	flags.BoolVar(&opts.authBrowserConsentAutoOpenBrowser, FlagAuthBrowserConsentAutoOpenBrowser, opts.authBrowserConsentAutoOpenBrowser, "automatically open browser for auth consent")
	flags.StringVar(&opts.credentialsStorageDir, FlagCredentialsStorageDir, opts.credentialsStorageDir, "credential storage directory; __default__ uses the platform default")
	if !DevFlagsVisible() {
		for _, name := range devFlagNames {
			if flag := flags.Lookup(name); flag != nil {
				flag.Hidden = true
			}
		}
	}
}

func DevFlagsVisible() bool {
	value := strings.TrimSpace(os.Getenv(ShowDevFlagsEnvVar))
	switch strings.ToLower(value) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func OverridesFromCommand(cmd *cobra.Command) (configuration.OverrideOptions, error) {
	var overrides configuration.OverrideOptions
	flags := cmd.Root().PersistentFlags()
	var err error

	overrides.AuthBaseURL, err = changedStr(flags, FlagAuthBaseURL)
	if err != nil {
		return overrides, err
	}
	overrides.GatewayBaseURL, err = changedStr(flags, FlagGatewayBaseURL)
	if err != nil {
		return overrides, err
	}
	overrides.AuthRetryOnOBORequired, err = changedValue(flags, FlagAuthRetryOnOBORequired, flags.GetBool)
	if err != nil {
		return overrides, err
	}
	overrides.AuthTokenTTLSeconds, err = changedValue(flags, FlagAuthTokenTTLSeconds, flags.GetInt32)
	if err != nil {
		return overrides, err
	}
	overrides.AuthBrowserConsentCallbackAddress, err = changedStr(flags, FlagAuthBrowserConsentCallbackAddress)
	if err != nil {
		return overrides, err
	}
	overrides.AuthBrowserConsentTimeout, err = changedValue(flags, FlagAuthBrowserConsentTimeout, flags.GetDuration)
	if err != nil {
		return overrides, err
	}
	overrides.AuthBrowserConsentAutoOpenBrowser, err = changedValue(flags, FlagAuthBrowserConsentAutoOpenBrowser, flags.GetBool)
	if err != nil {
		return overrides, err
	}
	overrides.CredentialsStorageDir, err = changedStr(flags, FlagCredentialsStorageDir)
	if err != nil {
		return overrides, err
	}

	return overrides, nil
}

func changedValue[T flagValue](flags *pflag.FlagSet, name string, get func(string) (T, error)) (*T, error) {
	if !flags.Changed(name) {
		return nil, nil
	}
	value, err := get(name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return &value, nil
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

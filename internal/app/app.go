package app

import (
	"fmt"
	"sync"

	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/client/auth"
	"github.com/tollbit/cli/internal/client/tollbit"
	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/credentials/agenttoken"
)

type Dependencies struct {
	Auth          *auth.Client
	Tollbit       tollbit.Client
	OBOAuthorizer agentauth.OBOAuthorizer
	Credentials   *agenttoken.CredentialManager
}

// App represents the application and its shared components.
type App struct {
	config configuration.Config
	deps   Dependencies

	auth          func() (*auth.Client, error)
	tollbit       func() (tollbit.Client, error)
	oboAuthorizer func() (agentauth.OBOAuthorizer, error)
	credentials   func() (*agenttoken.CredentialManager, error)
}

func New(config configuration.Config, opts ...Option) (*App, error) {
	var cfg options
	for _, opt := range opts {
		opt(&cfg)
	}
	a := &App{
		config: config,
		deps:   cfg.dependencies,
	}
	a.auth = sync.OnceValues(a.buildAuth)
	a.tollbit = sync.OnceValues(a.buildTollbit)
	a.oboAuthorizer = sync.OnceValues(a.buildOBOAuthorizer)
	a.credentials = sync.OnceValues(a.buildCredentials)
	return a, nil
}

func (a *App) Config() configuration.Config {
	return a.config
}

func (a *App) Auth() (*auth.Client, error) {
	return a.auth()
}

func (a *App) buildAuth() (*auth.Client, error) {
	if a.deps.Auth != nil {
		return a.deps.Auth, nil
	}
	client, err := auth.New(auth.ClientConfig{BaseURL: a.config.Auth.BaseURL})
	if err != nil {
		return nil, fmt.Errorf("build auth client: %w", err)
	}
	return client, nil
}

func (a *App) Tollbit() (tollbit.Client, error) {
	return a.tollbit()
}

func (a *App) buildTollbit() (tollbit.Client, error) {
	if a.deps.Tollbit != nil {
		return a.deps.Tollbit, nil
	}
	client, err := tollbit.NewClient(tollbit.Config{BaseURL: a.config.Gateway.BaseURL})
	if err != nil {
		return nil, fmt.Errorf("build tollbit client: %w", err)
	}
	return client, nil
}

func (a *App) OBOAuthorizer() (agentauth.OBOAuthorizer, error) {
	return a.oboAuthorizer()
}

func (a *App) buildOBOAuthorizer() (agentauth.OBOAuthorizer, error) {
	if a.deps.OBOAuthorizer != nil {
		return a.deps.OBOAuthorizer, nil
	}
	authClient, err := a.Auth()
	if err != nil {
		return nil, err
	}
	browserConsent := a.config.Auth.BrowserConsent
	authorizer, err := agentauth.NewBrowserConsentAuthorizer(agentauth.BrowserConsentAuthorizerConfig{
		AuthClient:       authClient,
		CallbackAddress:  browserConsent.CallbackAddress,
		AutoOpenBrowser:  browserConsent.AutoOpenBrowser,
		Timeout:          browserConsent.Timeout,
		UseRefreshTokens: a.config.Auth.UseRefreshTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("build OBO authorizer: %w", err)
	}
	return authorizer, nil
}

func (a *App) Credentials() (*agenttoken.CredentialManager, error) {
	return a.credentials()
}

func (a *App) buildCredentials() (*agenttoken.CredentialManager, error) {
	if a.deps.Credentials != nil {
		return a.deps.Credentials, nil
	}
	authClient, err := a.Auth()
	if err != nil {
		return nil, err
	}
	oboAuthorizer, err := a.OBOAuthorizer()
	if err != nil {
		return nil, err
	}
	tokenOptions := auth.AgentTokenOptions{}
	if a.config.Auth.TokenTTLSeconds > 0 {
		tokenOptions.TTLSeconds = &a.config.Auth.TokenTTLSeconds
	}
	credentials, err := agenttoken.New(agenttoken.CredentialManagerConfig{
		Path: a.config.Credentials.StorageDir,
		DefaultIdentity: auth.AgentIdentity{
			Name:      a.config.Agent.DefaultName,
			UserAgent: a.config.Agent.DefaultUserAgent,
		},
		TokenOptions:     tokenOptions,
		AuthClient:       authClient,
		OBOAuthorizer:    oboAuthorizer,
		UseRefreshTokens: a.config.Auth.UseRefreshTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("build credentials: %w", err)
	}
	return credentials, nil
}

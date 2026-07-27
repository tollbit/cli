package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/agentauth/browserselecticon"
	"github.com/tollbit/cli/internal/agentauth/redirect"
	"github.com/tollbit/cli/internal/client/auth"
	"github.com/tollbit/cli/internal/client/tollbit"
	"github.com/tollbit/cli/internal/cliruntime"
	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/credentials/agenttoken"
)

type Dependencies struct {
	Auth          *auth.Client
	Tollbit       tollbit.Client
	OBOAuthorizer agentauth.OBOAuthorizer
	Credentials   *agenttoken.CredentialManager
	Runtime       *cliruntime.Runtime
}

// App represents the application and its shared components.
type App struct {
	config configuration.Config
	deps   Dependencies

	auth          func() (*auth.Client, error)
	tollbit       func() (tollbit.Client, error)
	oboAuthorizer func() (agentauth.OBOAuthorizer, error)
	credentials   func() (*agenttoken.CredentialManager, error)
	runtime       func() (*cliruntime.Runtime, error)
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
	a.runtime = sync.OnceValues(a.buildRuntime)
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

func (a *App) Runtime() (*cliruntime.Runtime, error) {
	return a.runtime()
}

func (a *App) buildRuntime() (*cliruntime.Runtime, error) {
	if a.deps.Runtime != nil {
		return a.deps.Runtime, nil
	}
	rt, err := cliruntime.New(cliruntime.Config{
		ConfiguredEndUserProximity: a.config.Runtime.EndUserProximity,
		StateDir:                   a.config.Runtime.StateDir,
	})
	if err != nil {
		return nil, fmt.Errorf("build cli runtime: %w", err)
	}
	return rt, nil
}

func (a *App) buildOBOAuthorizer() (agentauth.OBOAuthorizer, error) {
	if a.deps.OBOAuthorizer != nil {
		return a.deps.OBOAuthorizer, nil
	}
	authClient, err := a.Auth()
	if err != nil {
		return nil, err
	}
	strategyName, err := a.ResolveConsentStrategy(context.Background())
	if err != nil {
		return nil, err
	}
	strategy, err := a.buildConsentStrategy(context.Background(), authClient, strategyName)
	if err != nil {
		return nil, err
	}

	authorizer, err := agentauth.NewConfiguredOBOAuthorizer(agentauth.ConfiguredOBOAuthorizerConfig{
		Strategy: strategy,
	})
	if err != nil {
		return nil, fmt.Errorf("build configured obo authorizer: %w", err)
	}

	return authorizer, nil
}

func (a *App) ResolveConsentStrategy(ctx context.Context) (string, error) {
	rt, err := a.Runtime()
	if err != nil {
		return "", err
	}
	proximity, err := rt.EndUserProximity(ctx)
	if err != nil {
		return "", err
	}
	switch proximity {
	case cliruntime.EndUserProximityLocal:
		return a.config.Auth.Consent.Strategy.Local, nil
	case cliruntime.EndUserProximityRemote:
		return a.config.Auth.Consent.Strategy.Remote, nil
	default:
		return "", fmt.Errorf("unsupported runtime end-user proximity %q", proximity)
	}
}

func (a *App) ConsentStrategy(method agentauth.ConsentMethod) (agentauth.ConsentStrategy, error) {
	authClient, err := a.Auth()
	if err != nil {
		return nil, err
	}
	return a.buildConsentStrategy(context.Background(), authClient, string(method))
}

func (a *App) buildConsentStrategy(ctx context.Context, authClient *auth.Client, strategy string) (agentauth.ConsentStrategy, error) {
	browserConsent := a.config.Auth.BrowserConsent
	rt, err := a.Runtime()
	if err != nil {
		return nil, err
	}
	proximity, err := rt.EndUserProximity(ctx)
	if err != nil {
		return nil, err
	}
	autoOpenBrowser := browserConsent.AutoOpenBrowser && proximity == cliruntime.EndUserProximityLocal
	switch strategy {
	case configuration.ConsentStrategyRedirect:
		redirectStrategy, err := redirect.NewConsentStrategy(redirect.ConsentStrategyConfig{
			AuthClient:       authClient,
			Runtime:          rt,
			CallbackAddress:  browserConsent.CallbackAddress,
			AutoOpenBrowser:  autoOpenBrowser,
			Timeout:          browserConsent.Timeout,
			UseRefreshTokens: a.config.Auth.UseRefreshTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("build redirect consent strategy: %w", err)
		}
		return redirectStrategy, nil
	case configuration.ConsentStrategyBrowserSelectIcon:
		browserSelectIconStrategy, err := browserselecticon.NewConsentStrategy(browserselecticon.ConsentStrategyConfig{
			AuthClient:       authClient,
			Runtime:          rt,
			AutoOpenBrowser:  autoOpenBrowser,
			UseRefreshTokens: a.config.Auth.UseRefreshTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("build browser-select-icon consent strategy: %w", err)
		}
		return browserSelectIconStrategy, nil
	default:
		return nil, fmt.Errorf("unsupported consent strategy %q", strategy)
	}
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

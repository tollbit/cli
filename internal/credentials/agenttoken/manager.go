package agenttoken

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/client/auth"
)

const (
	identityFilename = "agent-identity.json"
	tokenFilename    = "agent-token.jwt"
	refreshFilename  = "refresh-token.json"
)

type (
	GetAgentTokenOption func(*getAgentTokenOptions)

	getAgentTokenOptions struct {
		requireOBO       bool
		useRefreshTokens bool
	}

	ResolveIdentityOptions struct {
		Name      *string
		UserAgent *string
	}

	CredentialManagerConfig struct {
		Path             string
		DefaultIdentity  auth.AgentIdentity
		TokenOptions     auth.AgentTokenOptions
		AuthClient       *auth.Client
		OBOAuthorizer    agentauth.OBOAuthorizer
		UseRefreshTokens bool
	}

	// TODO: Fold persisted agent identity into the main credentials object instead of storing a separate file.
	CredentialManager struct {
		dir              string
		identityPath     string
		tokenPath        string
		refreshPath      string
		defaultIdentity  auth.AgentIdentity
		tokenOptions     auth.AgentTokenOptions
		authClient       *auth.Client
		oboAuthorizer    agentauth.OBOAuthorizer
		useRefreshTokens bool
	}
)

func WithOBO() GetAgentTokenOption {
	return func(opts *getAgentTokenOptions) {
		opts.requireOBO = true
	}
}

func WithRefreshTokens(enabled bool) GetAgentTokenOption {
	return func(opts *getAgentTokenOptions) {
		opts.useRefreshTokens = enabled
	}
}

func New(cfg CredentialManagerConfig) (*CredentialManager, error) {
	if cfg.AuthClient == nil {
		return nil, errors.New("auth client is required")
	}
	if cfg.OBOAuthorizer == nil {
		return nil, errors.New("OBO authorizer is required")
	}
	if strings.TrimSpace(cfg.Path) == "" {
		return nil, errors.New("credential path is required")
	}
	if strings.TrimSpace(cfg.DefaultIdentity.Name) == "" {
		return nil, errors.New("default agent identity name is required")
	}
	dir := strings.TrimSpace(cfg.Path)
	defaultIdentity := auth.AgentIdentity{
		Name:      strings.TrimSpace(cfg.DefaultIdentity.Name),
		UserAgent: strings.TrimSpace(cfg.DefaultIdentity.UserAgent),
		WBA:       cfg.DefaultIdentity.WBA,
	}

	return &CredentialManager{
		dir:              dir,
		identityPath:     filepath.Join(dir, identityFilename),
		tokenPath:        filepath.Join(dir, tokenFilename),
		refreshPath:      filepath.Join(dir, refreshFilename),
		defaultIdentity:  defaultIdentity,
		tokenOptions:     cfg.TokenOptions,
		authClient:       cfg.AuthClient,
		oboAuthorizer:    cfg.OBOAuthorizer,
		useRefreshTokens: cfg.UseRefreshTokens,
	}, nil
}

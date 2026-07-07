package agenttoken

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/tollbit/tollbit-cli/internal/agentauth"
	"github.com/tollbit/tollbit-cli/internal/client/auth"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
)

const (
	identityFilename = "agent-identity.json"
	tokenFilename    = "agent-token.jwt"
	EnvAgentToken    = "TOLLBIT_AGENT_TOKEN"
)

type (
	GetAgentTokenOption func(*getAgentTokenOptions)

	getAgentTokenOptions struct {
		requireOBO bool
	}

	ResolveIdentityOptions struct {
		Name      *string
		UserAgent *string
	}

	PatchIdentityOptions struct {
		Name      *string
		UserAgent *string
	}

	PatchIdentityResult struct {
		Identity    auth.AgentIdentity
		NameChanged bool
	}

	CredentialManagerConfig struct {
		Path            string
		DefaultIdentity auth.AgentIdentity
		TokenOptions    auth.AgentTokenOptions
		AuthClient      *auth.Client
		OBOAuthorizer   agentauth.OBOAuthorizer
	}

	// TODO: Fold persisted agent identity into the main credentials object instead of storing a separate file.
	CredentialManager struct {
		dir             string
		identityPath    string
		tokenPath       string
		defaultIdentity auth.AgentIdentity
		tokenOptions    auth.AgentTokenOptions
		authClient      *auth.Client
		oboAuthorizer   agentauth.OBOAuthorizer
	}
)

func WithOBO() GetAgentTokenOption {
	return func(opts *getAgentTokenOptions) {
		opts.requireOBO = true
	}
}

func New(cfg CredentialManagerConfig) (*CredentialManager, error) {
	if cfg.AuthClient == nil {
		return nil, errors.New("auth client is required")
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
	}

	return &CredentialManager{
		dir:             dir,
		identityPath:    filepath.Join(dir, identityFilename),
		tokenPath:       filepath.Join(dir, tokenFilename),
		defaultIdentity: defaultIdentity,
		tokenOptions:    cfg.TokenOptions,
		authClient:      cfg.AuthClient,
		oboAuthorizer:   cfg.OBOAuthorizer,
	}, nil
}

func (m *CredentialManager) SaveIdentity(ctx context.Context, identity auth.AgentIdentity) error {
	if err := m.WriteIdentity(ctx, identity); err != nil {
		return err
	}
	return m.ClearAgentTokens(ctx)
}

func (m *CredentialManager) WriteIdentity(ctx context.Context, identity auth.AgentIdentity) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(identity.Name) == "" {
		return errors.New("agent name is required")
	}
	identity.Name = strings.TrimSpace(identity.Name)
	identity.UserAgent = strings.TrimSpace(identity.UserAgent)

	if err := m.writeJSON(ctx, m.identityPath, identity); err != nil {
		return fmt.Errorf("save agent identity credential: %w", err)
	}
	return nil
}

func (m *CredentialManager) PatchIdentity(ctx context.Context, opts PatchIdentityOptions) (PatchIdentityResult, error) {
	if opts.Name == nil && opts.UserAgent == nil {
		return PatchIdentityResult{}, errors.New("at least one of name or user-agent is required")
	}
	if err := ctx.Err(); err != nil {
		return PatchIdentityResult{}, err
	}

	stored, storedExists, err := m.GetStoredIdentity(ctx)
	if err != nil {
		return PatchIdentityResult{}, err
	}

	base := m.defaultIdentity
	if storedExists {
		base = stored
	}

	merged := base
	if opts.Name != nil {
		merged.Name = strings.TrimSpace(*opts.Name)
	}
	if opts.UserAgent != nil {
		merged.UserAgent = strings.TrimSpace(*opts.UserAgent)
	}
	if merged.Name == "" {
		return PatchIdentityResult{}, errors.New("agent name is required")
	}

	nameChanged := false
	if opts.Name != nil {
		if storedExists {
			nameChanged = merged.Name != stored.Name
		} else {
			token, exists, tokenErr := m.cachedAgentTokenFromDisk(ctx)
			if exists && tokenErr == nil {
				if claims, claimsErr := token.Claims(); claimsErr == nil && claims.Subject != merged.Name {
					nameChanged = true
				}
			}
		}
	}

	if err := m.writeJSON(ctx, m.identityPath, merged); err != nil {
		return PatchIdentityResult{}, fmt.Errorf("save agent identity credential: %w", err)
	}
	if nameChanged {
		if err := m.ClearAgentTokens(ctx); err != nil {
			return PatchIdentityResult{}, err
		}
	}
	return PatchIdentityResult{Identity: merged, NameChanged: nameChanged}, nil
}

func (m *CredentialManager) GetIdentity(ctx context.Context) (auth.AgentIdentity, error) {
	identity, exists, err := m.GetStoredIdentity(ctx)
	if err != nil {
		return auth.AgentIdentity{}, err
	}
	if exists {
		return identity, nil
	}
	return m.defaultIdentity, nil
}

func (m *CredentialManager) ResolveIdentity(ctx context.Context, opts ResolveIdentityOptions) (auth.AgentIdentity, error) {
	identity, err := m.GetIdentity(ctx)
	if err != nil {
		return auth.AgentIdentity{}, err
	}
	if opts.Name != nil {
		identity.Name = *opts.Name
	}
	if opts.UserAgent != nil {
		identity.UserAgent = *opts.UserAgent
	}
	identity.Name = strings.TrimSpace(identity.Name)
	identity.UserAgent = strings.TrimSpace(identity.UserAgent)
	if identity.Name == "" {
		return auth.AgentIdentity{}, errors.New("agent name is required")
	}
	return identity, nil
}

func (m *CredentialManager) GetStoredIdentity(ctx context.Context) (auth.AgentIdentity, bool, error) {
	if err := ctx.Err(); err != nil {
		return auth.AgentIdentity{}, false, err
	}
	raw, err := os.ReadFile(m.identityPath)
	if os.IsNotExist(err) {
		return auth.AgentIdentity{}, false, nil
	}
	if err != nil {
		return auth.AgentIdentity{}, false, fmt.Errorf("read agent identity credential: %w", err)
	}
	var identity auth.AgentIdentity
	if err := json.Unmarshal(raw, &identity); err != nil {
		return auth.AgentIdentity{}, false, fmt.Errorf("parse agent identity credential: %w", err)
	}
	identity.Name = strings.TrimSpace(identity.Name)
	identity.UserAgent = strings.TrimSpace(identity.UserAgent)
	if identity.Name == "" {
		return auth.AgentIdentity{}, false, errors.New("agent identity credential missing name")
	}
	return identity, true, nil
}

func (m *CredentialManager) ClearIdentity(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := os.Remove(m.identityPath)
	if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("clear agent identity credential: %w", err)
	}
	return m.ClearAgentTokens(ctx)
}

func (m *CredentialManager) GetAgentToken(inv agentauth.Invocation, identity auth.AgentIdentity, options ...GetAgentTokenOption) (agent.Token, error) {
	ctx := inv.Context()
	l := zerolog.Ctx(ctx)

	if err := ctx.Err(); err != nil {
		return agent.Token{}, err
	}
	opts := getAgentTokenOptions{}
	for _, apply := range options {
		apply(&opts)
	}

	token, exists, err := m.cachedAgentToken(ctx)
	if err != nil {
		it := new(agent.InvalidTokenErr)
		if errors.As(err, &it) {
			l.Debug().Err(err).Msg("invalid token fetching new token")
			exists = false
		} else {
			return agent.Token{}, err
		}
	}
	if exists {
		if opts.requireOBO {
			claims, err := token.Claims()
			if err != nil {
				return agent.Token{}, err
			}
			if claims.OBO != nil {
				l.Debug().Str("path", m.tokenPath).Str("agent", identity.Name).Msg("OBO agent token loaded from cache")
				return token, nil
			}
		} else {
			l.Debug().Str("path", m.tokenPath).Str("agent", identity.Name).Msg("agent token loaded from cache")
			return token, nil
		}
	}

	if !exists {
		l.Debug().
			Str("path", m.tokenPath).
			Str("agent", identity.Name).
			Msg("agent token cache miss; minting via auth")
		token, err = m.createAndSave(ctx, identity)
		if err != nil {
			return agent.Token{}, err
		}
	}

	if opts.requireOBO {
		if m.oboAuthorizer == nil {
			return agent.Token{}, errors.New("OBO authorizer is required")
		}
		oboToken, err := m.oboAuthorizer.AuthorizeOBO(inv, identity, token)
		if err != nil {
			return agent.Token{}, err
		}
		if err := oboToken.Validate(); err != nil {
			return agent.Token{}, err
		}
		if err := m.write(ctx, oboToken); err != nil {
			return agent.Token{}, err
		}
		return oboToken, nil
	}
	return token, nil
}

func (m *CredentialManager) CurrentAgentToken(ctx context.Context) (agent.Token, bool, error) {
	return m.cachedAgentToken(ctx)
}

func (m *CredentialManager) cachedAgentToken(ctx context.Context) (agent.Token, bool, error) {
	if err := ctx.Err(); err != nil {
		return agent.Token{}, false, err
	}
	if raw := strings.TrimSpace(os.Getenv(EnvAgentToken)); raw != "" {
		token := agent.Token{RawToken: raw}
		err := token.Validate()
		return token, true, err
	}
	return m.cachedAgentTokenFromDisk(ctx)
}

func (m *CredentialManager) cachedAgentTokenFromDisk(ctx context.Context) (agent.Token, bool, error) {
	if err := ctx.Err(); err != nil {
		return agent.Token{}, false, err
	}
	raw, err := os.ReadFile(m.tokenPath)
	if os.IsNotExist(err) {
		return agent.Token{}, false, nil
	}
	if err != nil {
		return agent.Token{}, false, fmt.Errorf("read agent token credential: %w", err)
	}
	token := agent.Token{RawToken: strings.TrimSpace(string(raw))}
	err = token.Validate()
	return token, true, err
}

func (m *CredentialManager) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := os.Remove(m.tokenPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("clear agent token credential: %w", err)
	}
	return nil
}

func (m *CredentialManager) ClearAgentTokens(ctx context.Context) error {
	return m.Clear(ctx)
}

func (m *CredentialManager) createAndSave(ctx context.Context, identity auth.AgentIdentity) (agent.Token, error) {
	if err := ctx.Err(); err != nil {
		return agent.Token{}, err
	}
	zerolog.Ctx(ctx).Debug().
		Str("agent", identity.Name).
		Str("user_agent", identity.UserAgent).
		Msg("creating agent token")
	token, err := m.authClient.CreateAgentToken(ctx, identity, m.tokenOptions)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Str("agent", identity.Name).Msg("agent token mint failed")
		return agent.Token{}, err
	}
	if err := token.Validate(); err != nil {
		return agent.Token{}, err
	}
	if err := ctx.Err(); err != nil {
		return agent.Token{}, err
	}
	if err := m.write(ctx, token); err != nil {
		return agent.Token{}, err
	}
	return token, nil
}

func (m *CredentialManager) write(ctx context.Context, token agent.Token) error {
	return m.writeRaw(ctx, m.tokenPath, []byte(token.RawToken))
}

func (m *CredentialManager) writeJSON(ctx context.Context, path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return m.writeRaw(ctx, path, b)
}

func (m *CredentialManager) writeRaw(ctx context.Context, path string, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create agent credential directory: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	f, err := os.CreateTemp(filepath.Dir(path), ".agent-credential-*.tmp")
	if err != nil {
		return fmt.Errorf("create agent credential temp file: %w", err)
	}
	tmp := f.Name()
	defer func() {
		_ = os.Remove(tmp)
	}()

	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return fmt.Errorf("set agent credential temp file permissions: %w", err)
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return fmt.Errorf("write agent credential: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close agent credential temp file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("save agent credential: %w", err)
	}
	return nil
}

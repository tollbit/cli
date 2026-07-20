package agenttoken

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tollbit/cli/internal/client/auth"
)

func (m *CredentialManager) SaveIdentity(ctx context.Context, identity auth.AgentIdentity) error {
	if err := m.WriteIdentity(ctx, identity); err != nil {
		return err
	}
	return m.ClearAuthTokens(ctx, false)
}

func (m *CredentialManager) WriteIdentity(ctx context.Context, identity auth.AgentIdentity) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var err error
	if identity, err = validIdentity(identity); err != nil {
		return err
	}

	if err := m.writeJSON(ctx, m.identityPath, identity); err != nil {
		return fmt.Errorf("save agent identity credential: %w", err)
	}
	return nil
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

	if identity, err = validIdentity(identity); err != nil {
		return auth.AgentIdentity{}, err
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

	if identity, err = validIdentity(identity); err != nil {
		return auth.AgentIdentity{}, false, fmt.Errorf("stored identity is invalid: %w", err)
	}
	return identity, true, nil
}

func (m *CredentialManager) ClearIdentity(ctx context.Context, force bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	clearErr := m.ClearAuthTokens(ctx, force)
	// Fail closed: revoke failed without force → tokens were left intact, so
	// leave the identity in place too and let the whole logout be retried.
	if clearErr != nil && errors.Is(clearErr, ErrRevokeFailed) && !force {
		return clearErr
	}
	// A non-revoke error is a real filesystem failure — surface it.
	if clearErr != nil && !errors.Is(clearErr, ErrRevokeFailed) {
		return clearErr
	}
	// Tokens were cleared (success, or force): remove the identity file.
	if err := os.Remove(m.identityPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear agent identity credential: %w", err)
	}
	return clearErr // nil on full success; ErrRevokeFailed signal on force
}

func validIdentity(id auth.AgentIdentity) (auth.AgentIdentity, error) {
	if strings.TrimSpace(id.Name) == "" {
		return auth.AgentIdentity{}, errors.New("agent name is required")
	}
	id.Name = strings.TrimSpace(id.Name)
	id.UserAgent = strings.TrimSpace(id.UserAgent)

	return id, nil
}

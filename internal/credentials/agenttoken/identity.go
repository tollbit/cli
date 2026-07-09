package agenttoken

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tollbit/tollbit-cli/internal/client/auth"
)

func (m *CredentialManager) SaveIdentity(ctx context.Context, identity auth.AgentIdentity) error {
	if err := m.WriteIdentity(ctx, identity); err != nil {
		return err
	}
	return m.ClearAuthTokens(ctx)
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

func (m *CredentialManager) ClearIdentity(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.ClearAuthTokens(ctx); err != nil {
		return err
	}
	err := os.Remove(m.identityPath)
	if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("clear agent identity credential: %w", err)
	}
	return nil
}

func validIdentity(id auth.AgentIdentity) (auth.AgentIdentity, error) {
	if strings.TrimSpace(id.Name) == "" {
		return auth.AgentIdentity{}, errors.New("agent name is required")
	}
	id.Name = strings.TrimSpace(id.Name)
	id.UserAgent = strings.TrimSpace(id.UserAgent)

	return id, nil
}

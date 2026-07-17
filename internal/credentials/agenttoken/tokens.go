package agenttoken

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/tollbit/cli/internal/client/auth"
	"github.com/tollbit/cli/internal/tokens/agent"
)

func (m *CredentialManager) CurrentAgentToken(ctx context.Context) (agent.Token, bool, error) {
	return m.cachedAgentToken(ctx)
}

func (m *CredentialManager) createAgentToken(ctx context.Context, identity auth.AgentIdentity) (agent.Token, error) {
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
	return token, nil
}

func (m *CredentialManager) createAndSave(ctx context.Context, identity auth.AgentIdentity) (agent.Token, error) {
	token, err := m.createAgentToken(ctx, identity)
	if err != nil {
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

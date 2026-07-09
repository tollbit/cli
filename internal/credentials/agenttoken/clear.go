package agenttoken

import (
	"context"

	"github.com/rs/zerolog"
)

func (m *CredentialManager) Clear(ctx context.Context) error {
	return m.clearAgentToken(ctx)
}

func (m *CredentialManager) ClearAgentTokens(ctx context.Context) error {
	return m.ClearAuthTokens(ctx)
}

func (m *CredentialManager) ClearAuthTokens(ctx context.Context) error {
	refresh, exists, err := m.readRefreshCredential(ctx)
	if err != nil {
		return err
	}
	if exists {
		if err := m.revokeRefreshToken(ctx, refresh); err != nil {
			return err
		}
	}
	if err := m.clearAgentToken(ctx); err != nil {
		return err
	}
	return m.clearRefreshCredential(ctx)
}

func (m *CredentialManager) revokeRefreshToken(ctx context.Context, cred refreshCredential) error {
	resp, err := m.authClient.RevokeRefreshToken(ctx, cred.RefreshToken)
	if err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).
			Str("path", m.refreshPath).
			Str("refresh_token_expires_at", cred.RefreshTokenExpiresAt).
			Msg("refresh token revocation failed")
		return err
	}
	zerolog.Ctx(ctx).Debug().
		Str("path", m.refreshPath).
		Str("refresh_token_expires_at", cred.RefreshTokenExpiresAt).
		Bool("revoked", resp.Revoked).
		Msg("refresh token revoked")
	return nil
}

package agenttoken

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
)

// ErrRevokeFailed indicates the refresh token could not be revoked on the
// server. Without force, ClearAuthTokens leaves local credentials intact so the
// user can retry logout. With force, local credentials are cleared and this is
// returned as a non-fatal signal so callers can warn about residual validity.
var ErrRevokeFailed = errors.New("refresh token could not be revoked on the server")

func (m *CredentialManager) Clear(ctx context.Context) error {
	return m.clearAgentToken(ctx)
}

func (m *CredentialManager) ClearAgentTokens(ctx context.Context, force bool) error {
	return m.ClearAuthTokens(ctx, force)
}

func (m *CredentialManager) ClearAuthTokens(ctx context.Context, force bool) error {
	refresh, exists, err := m.readRefreshCredential(ctx)
	if err != nil {
		return err
	}
	var revokeErr error
	if exists {
		revokeErr = m.revokeRefreshToken(ctx, refresh)
	}
	if revokeErr != nil && !force {
		// Fail closed: keep local credentials so logout can be retried.
		return fmt.Errorf("%w: %w", ErrRevokeFailed, revokeErr)
	}
	if err := m.clearAgentToken(ctx); err != nil {
		return err
	}
	if err := m.clearRefreshCredential(ctx); err != nil {
		return err
	}
	if revokeErr != nil {
		// force: local credentials cleared, but revocation failed — signal it.
		return fmt.Errorf("%w: %w", ErrRevokeFailed, revokeErr)
	}
	return nil
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

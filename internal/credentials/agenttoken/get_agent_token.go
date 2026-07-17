package agenttoken

import (
	"errors"

	"github.com/rs/zerolog"
	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/client/auth"
	"github.com/tollbit/cli/internal/tokens/agent"
)

func (m *CredentialManager) GetAgentToken(inv agentauth.Invocation, identity auth.AgentIdentity, options ...GetAgentTokenOption) (agent.Token, error) {
	ctx := inv.Context()
	l := zerolog.Ctx(ctx)

	if err := ctx.Err(); err != nil {
		return agent.Token{}, err
	}
	opts := getAgentTokenOptions{useRefreshTokens: m.useRefreshTokens}
	for _, apply := range options {
		apply(&opts)
	}

	// Start with the cached token. It may be a base agent token or an OBO token.
	token, exists, err := m.cachedAgentToken(ctx)
	if err != nil {
		it := new(agent.InvalidTokenErr)
		if errors.As(err, &it) {
			// Expired OBO tokens can be refreshed without user consent when a refresh token is available.
			if opts.requireOBO && opts.useRefreshTokens && exists && token.Expired() {
				obo, innerErr := hasObo(token)
				switch {
				case innerErr != nil:
					l.Debug().
						Err(innerErr).
						Msg("error checking token for OBO")
				case obo:
					if refreshed, ok := m.refreshOBOTokenViaRefreshToken(inv, identity); ok {
						return refreshed, nil
					}
				}
			}
			l.Debug().
				Err(err).
				Msg("invalid token fetching new token")
			exists = false
		} else {
			return agent.Token{}, err
		}
	}
	// A cached token is usable as-is unless this call specifically needs OBO.
	if exists {
		if opts.requireOBO {
			obo, err := hasObo(token)
			if err != nil {
				return agent.Token{}, err
			}
			if obo {
				l.Debug().
					Str("path", m.tokenPath).
					Str("agent", identity.Name).
					Msg("OBO agent token loaded from cache")
				return token, nil
			}
		} else {
			l.Debug().
				Str("path", m.tokenPath).
				Str("agent", identity.Name).
				Msg("agent token loaded from cache")
			return token, nil
		}
	}

	// No usable cached token: mint and persist a base agent token.
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

	// OBO is requested but the cached/minted token is not already OBO, so run browser consent.
	if opts.requireOBO {
		resp, err := m.oboAuthorizer.AuthorizeOBO(inv, identity, token)
		if err != nil {
			return agent.Token{}, err
		}
		oboToken := agent.Token{RawToken: resp.Token}
		if err := oboToken.Validate(); err != nil {
			return agent.Token{}, err
		}
		// Persist the OBO token and clear any stale refresh token unless this response includes a rotated one.
		creds := credentials{AgentToken: oboToken}
		if opts.useRefreshTokens {
			creds.RefreshToken = valueOrEmpty(resp.RefreshToken)
			creds.RefreshTokenExpiresAt = valueOrEmpty(resp.RefreshTokenExpiresAt)
		}
		if err := m.saveCredentials(ctx, creds); err != nil {
			return agent.Token{}, err
		}
		return oboToken, nil
	}
	return token, nil
}

// refreshOBOTokenViaRefreshToken tries the non-interactive refresh-token path.
// The caller only invokes this after confirming the cached token is expired and has OBO claims.
func (m *CredentialManager) refreshOBOTokenViaRefreshToken(inv agentauth.Invocation, identity auth.AgentIdentity) (agent.Token, bool) {
	ctx := inv.Context()
	l := zerolog.Ctx(ctx)

	// Without a stored refresh token we cannot refresh silently; fall back to browser consent.
	refresh, exists, err := m.readRefreshCredential(ctx)
	if err != nil {
		l.Debug().
			Err(err).
			Str("agent", identity.Name).
			Str("path", m.refreshPath).
			Str("fallback", "browser_consent").
			Msg("refresh token credential unavailable; falling back to browser consent")
		return agent.Token{}, false
	}
	if !exists {
		l.Debug().
			Str("agent", identity.Name).
			Str("path", m.refreshPath).
			Str("fallback", "browser_consent").
			Str("reason", "missing_refresh_token").
			Msg("no refresh token available; falling back to browser consent")
		return agent.Token{}, false
	}

	if expired, err := refresh.expired(); err != nil || expired {
		if err == nil {
			err = errors.New("refresh token has expired")
		}
		l.Debug().
			Err(err).
			Str("agent", identity.Name).
			Str("path", m.refreshPath).
			Str("fallback", "browser_consent").
			Str("refresh_token_expires_at", refresh.RefreshTokenExpiresAt).
			Msg("refresh token expired locally; clearing refresh token and falling back to browser consent")
		if clearErr := m.clearRefreshCredential(ctx); clearErr != nil {
			l.Debug().
				Err(clearErr).
				Str("agent", identity.Name).
				Str("path", m.refreshPath).
				Msg("failed to clear expired refresh token")
		}
		return agent.Token{}, false
	}

	l.Debug().
		Str("agent", identity.Name).
		Str("path", m.refreshPath).
		Str("refresh_token_expires_at", refresh.RefreshTokenExpiresAt).
		Msg("attempting refresh token grant")

	// The refresh-token grant is authenticated with a fresh base agent token.
	baseToken, err := m.createAgentToken(ctx, identity)
	if err != nil {
		l.Debug().
			Err(err).
			Str("agent", identity.Name).
			Str("fallback", "browser_consent").
			Msg("failed to mint base token for refresh token grant; falling back to browser consent")
		return agent.Token{}, false
	}

	// Refresh grants rotate the refresh token; save both credentials together.
	resp, err := m.authClient.RefreshAgentToken(ctx, baseToken, auth.RefreshTokenGrantRequest{
		AgentIdentifier: identity.Name,
		RefreshToken:    refresh.RefreshToken,
		UA:              userAgentPtr(identity),
		WBA:             identity.WBA,
	})
	if err != nil {
		l.Debug().
			Err(err).
			Str("agent", identity.Name).
			Str("path", m.refreshPath).
			Str("fallback", "browser_consent").
			Str("reason", "refresh_token_failed").
			Msg("refresh token grant failed; clearing refresh token and falling back to browser consent")
		// Do not retry with a failed refresh token; clear it and fall back to consent.
		if clearErr := m.clearRefreshCredential(ctx); clearErr != nil {
			l.Debug().
				Err(clearErr).
				Str("agent", identity.Name).
				Str("path", m.refreshPath).
				Msg("failed to clear refresh token after refresh grant error")
		} else {
			l.Debug().
				Str("agent", identity.Name).
				Str("path", m.refreshPath).
				Msg("cleared failed refresh token")
		}
		return agent.Token{}, false
	}

	refreshed := agent.Token{RawToken: resp.Token}
	if err := refreshed.Validate(); err != nil {
		l.Debug().
			Err(err).
			Str("agent", identity.Name).
			Str("fallback", "browser_consent").
			Msg("refreshed OBO token was invalid; clearing refresh token and falling back to browser consent")
		_ = m.clearRefreshCredential(ctx)
		return agent.Token{}, false
	}

	// Store the refreshed OBO token with the rotated refresh token from the response.
	if err := m.saveCredentials(ctx, credentials{
		AgentToken:            refreshed,
		RefreshToken:          valueOrEmpty(resp.RefreshToken),
		RefreshTokenExpiresAt: valueOrEmpty(resp.RefreshTokenExpiresAt),
	}); err != nil {
		l.Debug().
			Err(err).
			Str("agent", identity.Name).
			Str("path", m.refreshPath).
			Msg("failed to save refreshed OBO credentials")
		return agent.Token{}, false
	}
	l.Debug().
		Str("agent", identity.Name).
		Str("path", m.refreshPath).
		Str("refresh_token_expires_at", valueOrEmpty(resp.RefreshTokenExpiresAt)).
		Msg("refresh token grant succeeded; stored rotated refresh token")
	return refreshed, true
}

func hasObo(token agent.Token) (bool, error) {
	claims, err := token.Claims()
	if err != nil {
		return false, err
	}
	exists := claims.OBO != nil
	return exists, nil
}

func userAgentPtr(identity auth.AgentIdentity) *string {
	if identity.UserAgent == "" {
		return nil
	}
	return &identity.UserAgent
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

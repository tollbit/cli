package agenttoken

import "context"

func (m *CredentialManager) RefreshTokenStatus(ctx context.Context) RefreshTokenStatus {
	refresh, exists, err := m.readRefreshCredential(ctx)
	if err != nil {
		return RefreshTokenStatus{Error: err.Error()}
	}
	if !exists {
		return RefreshTokenStatus{Present: false}
	}
	expired, err := refresh.expired()
	if err != nil {
		return RefreshTokenStatus{Present: true, ExpiresAt: refresh.RefreshTokenExpiresAt, Expired: true, Error: err.Error()}
	}
	return RefreshTokenStatus{Present: true, ExpiresAt: refresh.RefreshTokenExpiresAt, Expired: expired}
}

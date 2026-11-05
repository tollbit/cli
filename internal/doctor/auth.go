package doctor

import (
	"context"

	"github.com/tollbit/cli/internal/credentials/agenttoken"
	"github.com/tollbit/cli/internal/tokens/agent"
)

// CredentialStore is the part of agenttoken.CredentialManager the auth
// check reads. Both methods read files only, so the check never calls the
// network or refreshes a token.
type CredentialStore interface {
	CurrentAgentToken(ctx context.Context) (agent.Token, bool, error)
	RefreshTokenStatus(ctx context.Context) agenttoken.RefreshTokenStatus
}

const loginHint = "run `tollbit auth login`"

// authCheck reports what is stored, never the token itself. It is
// informational: not being logged in is not an install problem.
func authCheck(ctx context.Context, store CredentialStore) Check {
	c := Check{Group: GroupAuth, Name: "credentials", Status: StatusInfo}
	token, stored, tokenErr := store.CurrentAgentToken(ctx)
	refresh := store.RefreshTokenStatus(ctx)
	if !stored && !refresh.Present {
		c.Detail = "not logged in"
		c.Hint = loginHint
		if refresh.Error != "" {
			c.Detail = "could not read stored credentials: " + refresh.Error
		}
		return c
	}

	c.Detail = "token stored"
	expiresAt, hasExpiry := token.ExpiresAt()
	when := expiresAt.Format("2006-01-02 15:04 MST")
	switch {
	case !stored:
		c.Detail = "no token stored"
	case tokenErr == nil && hasExpiry:
		c.Detail += ", expires " + when
	case token.Expired() && hasExpiry:
		c.Detail += ", expired " + when
	case tokenErr != nil:
		c.Detail += " but invalid (" + tokenErr.Error() + ")"
		c.Hint = loginHint
	}
	switch {
	case refresh.Error != "" && !refresh.Expired:
		c.Detail += "; refresh token unreadable (" + refresh.Error + ")"
		c.Hint = loginHint
	case refresh.Expired:
		c.Detail += "; refresh token expired"
		c.Hint = loginHint
	case refresh.Present:
		c.Detail += "; refresh token stored"
	}
	return c
}

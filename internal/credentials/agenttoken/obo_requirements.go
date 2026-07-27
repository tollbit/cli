package agenttoken

import (
	"errors"

	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/client/auth"
	"github.com/tollbit/cli/internal/errorsx/problemjson"
	"github.com/tollbit/cli/internal/tokens/agent"
)

type ExplicitLoginRequiredError struct {
	Reason string
}

func (e ExplicitLoginRequiredError) Error() string {
	if e.Reason != "" {
		return e.Reason
	}
	return "explicit auth login is required"
}

func WithOBORetry[T any](inv agentauth.Invocation, mgr *CredentialManager, identity auth.AgentIdentity, call func(agent.Token) (T, error)) (T, error) {
	token, err := mgr.GetAgentToken(inv, identity)
	if err != nil {
		var zero T
		return zero, err
	}
	out, err := call(token)
	if err == nil || !IsOBORequired(err) {
		return out, err
	}
	if !mgr.SupportsOBORetry() {
		return out, ExplicitLoginRequiredError{Reason: "operation requires user or organization authorization; run `tollbit auth login`, then retry"}
	}

	oboToken, err := mgr.GetAgentToken(inv, identity, WithOBO())
	if err != nil {
		var zero T
		return zero, err
	}
	return call(oboToken)
}

func IsOBORequired(err error) bool {
	var problem problemjson.Problem
	return errors.As(err, &problem) && problem.IsOBORequired()
}

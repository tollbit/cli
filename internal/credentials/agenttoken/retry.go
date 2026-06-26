package agenttoken

import (
	"errors"

	"github.com/tollbit/tollbit-cli/internal/agentauth"
	"github.com/tollbit/tollbit-cli/internal/client/auth"
	"github.com/tollbit/tollbit-cli/internal/errorsx/problemjson"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
)

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

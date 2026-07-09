package agentauth

import (
	"github.com/tollbit/tollbit-cli/internal/client/auth"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
)

type OBOAuthorizer interface {
	AuthorizeOBO(inv Invocation, identity auth.AgentIdentity, baseToken agent.Token) (auth.AgentTokenResponse, error)
}

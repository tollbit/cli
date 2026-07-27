package agentauth

import (
	"errors"
	"fmt"
	"time"

	"github.com/tollbit/cli/internal/client/auth"
	"github.com/tollbit/cli/internal/tokens/agent"
)

type ConsentMethod string

const (
	ConsentMethodRedirect          ConsentMethod = "redirect"
	ConsentMethodBrowserSelectIcon ConsentMethod = "browser_select_icon"
)

type (
	OBOAuthorizer interface {
		SupportsOBORetry() bool
		AuthorizeOBO(inv Invocation, identity auth.AgentIdentity, baseToken agent.Token) (auth.AgentTokenResponse, error)
		CompleteDetached(inv Invocation, pending PendingConsent, input CompleteDetachedInput) (auth.AgentTokenResponse, error)
	}

	ConsentStrategy interface {
		Method() ConsentMethod
		AutoOpensBrowser() bool
		SupportsOBORetry() bool
		SupportsDetachedCompletion() bool
		Guidance() ConsentGuidance
		AuthorizeOBO(inv Invocation, identity auth.AgentIdentity, baseToken agent.Token) (auth.AgentTokenResponse, error)
		CompleteDetached(inv Invocation, pending PendingConsent, input CompleteDetachedInput) (auth.AgentTokenResponse, error)
	}

	CompleteDetachedInput struct {
		IconNames []string
	}

	ConsentGuidance struct {
		FlowLabel            string
		LoginInstructions    string
		CompleteInstructions string
		CompleteArgsLabel    string
		Troubleshooting      []ConsentGuidanceRow
	}

	ConsentGuidanceRow struct {
		Symptom string
		Meaning string
		Action  string
	}

	PendingConsent struct {
		Method           ConsentMethod         `json:"method"`
		ChallengeID      string                `json:"challenge_id"`
		AgentIdentity    auth.AgentIdentity    `json:"agent_identity"`
		BaseToken        string                `json:"base_token"`
		CodeVerifier     string                `json:"code_verifier"`
		UseRefreshTokens bool                  `json:"use_refresh_tokens"`
		CreatedAt        time.Time             `json:"created_at"`
		ExpiresAt        string                `json:"expires_at,omitempty"`
		CorrectIcon      auth.AgentConsentIcon `json:"correct_icon"`
		IconNames        []string              `json:"icon_names,omitempty"`
	}

	AuthorizationPendingError struct {
		Pending PendingConsent
	}

	ConfiguredOBOAuthorizerConfig struct {
		Strategy ConsentStrategy
	}

	ConfiguredOBOAuthorizer struct {
		strategy ConsentStrategy
	}
)

func (e AuthorizationPendingError) Error() string {
	if e.Pending.ChallengeID == "" {
		return "authorization pending"
	}
	return fmt.Sprintf("authorization pending for challenge %s", e.Pending.ChallengeID)
}

func NewConfiguredOBOAuthorizer(cfg ConfiguredOBOAuthorizerConfig) (*ConfiguredOBOAuthorizer, error) {
	if cfg.Strategy == nil {
		return nil, errors.New("consent strategy is required")
	}
	return &ConfiguredOBOAuthorizer{strategy: cfg.Strategy}, nil
}

func (a ConfiguredOBOAuthorizer) SupportsOBORetry() bool {
	return a.strategy.SupportsOBORetry()
}

func (a ConfiguredOBOAuthorizer) AuthorizeOBO(inv Invocation, identity auth.AgentIdentity, baseToken agent.Token) (auth.AgentTokenResponse, error) {
	return a.strategy.AuthorizeOBO(inv, identity, baseToken)
}

func (a ConfiguredOBOAuthorizer) CompleteDetached(inv Invocation, pending PendingConsent, input CompleteDetachedInput) (auth.AgentTokenResponse, error) {
	return a.strategy.CompleteDetached(inv, pending, input)
}

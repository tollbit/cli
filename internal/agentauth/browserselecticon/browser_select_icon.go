package browserselecticon

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/agentauth/common"
	"github.com/tollbit/cli/internal/client/auth"
	"github.com/tollbit/cli/internal/cliruntime"
	"github.com/tollbit/cli/internal/tokens/agent"
)

type (
	ConsentStrategyConfig struct {
		AuthClient       *auth.Client
		Runtime          *cliruntime.Runtime
		AutoOpenBrowser  bool
		UseRefreshTokens bool
	}

	ConsentStrategy struct {
		authClient       *auth.Client
		runtime          *cliruntime.Runtime
		autoOpenBrowser  bool
		useRefreshTokens bool
	}
)

func NewConsentStrategy(cfg ConsentStrategyConfig) (*ConsentStrategy, error) {
	if cfg.AuthClient == nil {
		return nil, errors.New("auth client is required")
	}
	if cfg.Runtime == nil {
		return nil, errors.New("runtime is required")
	}
	return &ConsentStrategy{
		authClient:       cfg.AuthClient,
		runtime:          cfg.Runtime,
		autoOpenBrowser:  cfg.AutoOpenBrowser,
		useRefreshTokens: cfg.UseRefreshTokens,
	}, nil
}

func (s ConsentStrategy) Method() agentauth.ConsentMethod {
	return agentauth.ConsentMethodBrowserSelectIcon
}

func (s ConsentStrategy) AutoOpensBrowser() bool {
	return s.autoOpenBrowser
}

func (s ConsentStrategy) SupportsOBORetry() bool {
	return false
}

func (s ConsentStrategy) SupportsDetachedCompletion() bool {
	return true
}

func (s ConsentStrategy) Guidance() agentauth.ConsentGuidance {
	return agentauth.ConsentGuidance{
		FlowLabel:            "detached browser relay",
		LoginInstructions:    "For remote authorization, run `tollbit auth login`, then relay the printed URL and verification icon to the end user exactly as shown. Do not summarize, rename, or replace the icon.",
		CompleteInstructions: "After the end user finishes authorization in their browser, run `tollbit auth complete` with no arguments.",
		CompleteArgsLabel:    "pending auth",
		Troubleshooting: []agentauth.ConsentGuidanceRow{
			{Symptom: "authorization still pending", Meaning: "the end user has not finished browser consent", Action: "wait for the end user to finish, then rerun `tollbit auth complete`"},
			{Symptom: "authorization denied, expired, or invalid", Meaning: "the pending challenge can no longer be completed", Action: "restart with `tollbit auth login`"},
		},
	}
}

func (s ConsentStrategy) AuthorizeOBO(inv agentauth.Invocation, identity auth.AgentIdentity, baseToken agent.Token) (auth.AgentTokenResponse, error) {
	ctx := inv.Context()
	codeVerifier, codeChallenge, err := common.GeneratePKCE()
	if err != nil {
		return auth.AgentTokenResponse{}, err
	}
	scope := ""
	if s.useRefreshTokens {
		scope = "offline_access"
	}

	startResp, err := s.authClient.StartAgentConsentBrowserSelectIcon(ctx, baseToken, auth.ConsentBrowserSelectIconStartRequest{
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: common.PKCEChallengeMethod,
		Scope:               scope,
	})
	if err != nil {
		return auth.AgentTokenResponse{}, err
	}
	if strings.TrimSpace(startResp.ChallengeID) == "" {
		return auth.AgentTokenResponse{}, errors.New("auth did not return a consent challenge id")
	}
	if strings.TrimSpace(startResp.ConsentURL) == "" {
		return auth.AgentTokenResponse{}, errors.New("auth did not return a consent URL")
	}

	stdout := inv.OutOrStdout()
	fmt.Fprintf(stdout, "Authorize agent: %s\n\n", identity.Name)
	if s.AutoOpensBrowser() {
		if err := s.runtime.OpenBrowser(startResp.ConsentURL); err != nil {
			fmt.Fprintf(stdout, "Could not open your browser automatically: %v\n", err)
			fmt.Fprintln(stdout, "Open this URL in your browser to continue:")
		} else {
			fmt.Fprintln(stdout, "Opened authorization page in your browser.")
			fmt.Fprintln(stdout, "If it did not open, visit:")
		}
	} else {
		fmt.Fprintln(stdout, "Open this URL in the end user's browser to continue:")
	}
	fmt.Fprintf(stdout, "%s\n\n", startResp.ConsentURL)
	fmt.Fprintln(stdout, "Relay the following verification icon to the end user exactly. Do not summarize, rename, or replace it with an emoji. The name alone is not sufficient.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "BEGIN VERIFICATION ICON")
	fmt.Fprintf(stdout, "Name: %s\n", startResp.CorrectIcon.Name)
	if startResp.CorrectIcon.Art != "" {
		fmt.Fprintf(stdout, "```text\n%s\n```\n", startResp.CorrectIcon.Art)
	}
	fmt.Fprintln(stdout, "END VERIFICATION ICON")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "After the user completes authorization, run `tollbit auth complete`.")

	pending := agentauth.PendingConsent{
		Method:           agentauth.ConsentMethodBrowserSelectIcon,
		ChallengeID:      startResp.ChallengeID,
		AgentIdentity:    identity,
		BaseToken:        baseToken.RawToken,
		CodeVerifier:     codeVerifier,
		UseRefreshTokens: s.useRefreshTokens,
		CreatedAt:        time.Now().UTC(),
		ExpiresAt:        startResp.ExpiresAt,
		CorrectIcon:      startResp.CorrectIcon,
	}
	return auth.AgentTokenResponse{}, agentauth.AuthorizationPendingError{Pending: pending}
}

func (s ConsentStrategy) CompleteDetached(inv agentauth.Invocation, pending agentauth.PendingConsent, input agentauth.CompleteDetachedInput) (auth.AgentTokenResponse, error) {
	if pending.Method != agentauth.ConsentMethodBrowserSelectIcon {
		return auth.AgentTokenResponse{}, fmt.Errorf("pending consent method %q is not supported by browser-select-icon", pending.Method)
	}
	var ua *string
	if pending.AgentIdentity.UserAgent != "" {
		ua = &pending.AgentIdentity.UserAgent
	}
	return s.authClient.RedeemAgentConsentBrowserSelectIcon(inv.Context(), agent.Token{RawToken: pending.BaseToken}, auth.ConsentBrowserSelectIconTokenRequest{
		AgentIdentifier: pending.AgentIdentity.Name,
		ChallengeID:     pending.ChallengeID,
		CodeVerifier:    pending.CodeVerifier,
		UA:              ua,
		WBA:             pending.AgentIdentity.WBA,
	})
}

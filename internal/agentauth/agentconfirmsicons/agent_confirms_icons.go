package agentconfirmsicons

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

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
	return agentauth.ConsentMethodAgentConfirmsIcons
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
		FlowLabel:            "detached icon confirmation",
		LoginInstructions:    "For remote authorization, run `tollbit auth login` and relay only the printed consent URL to the end user. Do not send the valid icon vocabulary to the user; it is for the agent's spelling reference only.",
		CompleteInstructions: "After the end user authorizes in their browser, they will read back three icon names. Verify the icon names against the list of valid icons previously provided in `auth login`. Then run `tollbit auth complete <first> <second> <third>` using those names in exactly the displayed order.",
		CompleteArgsLabel:    "pending auth plus three icon names",
		Troubleshooting: []agentauth.ConsentGuidanceRow{
			{Symptom: "unrecognized icon name", Meaning: "one submitted word did not match the icon manifest and no attempt was consumed", Action: "confirm spelling with the end user using the printed vocabulary, then rerun `tollbit auth complete <first> <second> <third>`"},
			{Symptom: "authorization denied", Meaning: "the icons or order were wrong and the challenge is dead", Action: "restart with `tollbit auth login`"},
			{Symptom: "authorization still pending", Meaning: "the end user has not clicked Authorize yet", Action: "wait for the end user to finish, then rerun `tollbit auth complete <first> <second> <third>`"},
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

	startResp, err := s.authClient.StartAgentConsentAgentConfirmsIcons(ctx, baseToken, auth.ConsentAgentConfirmsIconsStartRequest{
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
	if len(startResp.IconNames) == 0 {
		return auth.AgentTokenResponse{}, errors.New("auth did not return valid icon names")
	}

	stdout := inv.OutOrStdout()
	fmt.Fprintf(stdout, "Authorize agent: %s\n\n", identity.Name)
	if s.AutoOpensBrowser() {
		if err := s.runtime.OpenBrowser(startResp.ConsentURL); err != nil {
			fmt.Fprintf(stdout, "Could not open your browser automatically: %v\n", err)
			fmt.Fprintln(stdout, "Open this URL in the end user's browser to continue:")
		} else {
			fmt.Fprintln(stdout, "Opened authorization page in your browser.")
			fmt.Fprintln(stdout, "If it did not open, visit:")
		}
	} else {
		fmt.Fprintln(stdout, "Relay ONLY this URL to the end user. The end user must open it in their browser, sign in, and click Authorize:")
	}
	fmt.Fprintf(stdout, "%s\n\n", startResp.ConsentURL)
	fmt.Fprintln(stdout, "VALID ICON NAMES (for the agent's reference; use to correct the user's spelling; do NOT send this list to the user):")
	fmt.Fprintln(stdout, strings.Join(startResp.IconNames, " "))
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "After the user reads back the three icons shown on the page, run `tollbit auth complete <first> <second> <third>`.")
	fmt.Fprintln(stdout, "Order matters.")

	pending := agentauth.PendingConsent{
		Method:           agentauth.ConsentMethodAgentConfirmsIcons,
		ChallengeID:      startResp.ChallengeID,
		AgentIdentity:    identity,
		BaseToken:        baseToken.RawToken,
		CodeVerifier:     codeVerifier,
		UseRefreshTokens: s.useRefreshTokens,
		CreatedAt:        time.Now().UTC(),
		ExpiresAt:        startResp.ExpiresAt,
		IconNames:        startResp.IconNames,
	}
	return auth.AgentTokenResponse{}, agentauth.AuthorizationPendingError{Pending: pending}
}

func (s ConsentStrategy) CompleteDetached(inv agentauth.Invocation, pending agentauth.PendingConsent, input agentauth.CompleteDetachedInput) (auth.AgentTokenResponse, error) {
	if pending.Method != agentauth.ConsentMethodAgentConfirmsIcons {
		return auth.AgentTokenResponse{}, fmt.Errorf("pending consent method %q is not supported by agent-confirms-icons", pending.Method)
	}
	iconNames := normalizeIconNames(input.IconNames)
	if len(iconNames) != 3 {
		return auth.AgentTokenResponse{}, fmt.Errorf("expected exactly 3 icon names; run `tollbit auth complete <first> <second> <third>` in the order shown to the user. Valid icon names: %s", strings.Join(pending.IconNames, " "))
	}
	var ua *string
	if pending.AgentIdentity.UserAgent != "" {
		ua = &pending.AgentIdentity.UserAgent
	}
	return s.authClient.RedeemAgentConsentAgentConfirmsIcons(inv.Context(), agent.Token{RawToken: pending.BaseToken}, auth.ConsentAgentConfirmsIconsTokenRequest{
		AgentIdentifier: pending.AgentIdentity.Name,
		ChallengeID:     pending.ChallengeID,
		IconNames:       iconNames,
		CodeVerifier:    pending.CodeVerifier,
		UA:              ua,
		WBA:             pending.AgentIdentity.WBA,
	})
}

func normalizeIconNames(input []string) []string {
	iconNames := make([]string, 0, len(input))
	for _, name := range input {
		name = strings.TrimFunc(strings.TrimSpace(name), func(r rune) bool {
			return unicode.IsPunct(r) || unicode.IsSpace(r)
		})
		name = strings.ToUpper(name)
		if name != "" {
			iconNames = append(iconNames, name)
		}
	}
	return iconNames
}

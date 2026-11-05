package codeauthorization

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
	"github.com/tollbit/cli/internal/errorsx/problemjson"
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

const (
	codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	codeLength   = 8
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
	return agentauth.ConsentMethodCodeAuthorization
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
		FlowLabel:            "detached code authorization",
		LoginInstructions:    "For remote authorization, run `tollbit auth login` and relay only the printed consent URL to the end user. After they sign in and click Authorize, the page shows a confirmation code in the form XXXX-XXXX. Ask them to paste it back to you.",
		CompleteInstructions: "Run `tollbit auth complete <code>` with the code the end user pasted back. Case, hyphens, and spaces do not matter. A wrong code ends the authorization, so use the code exactly as given. The code is a secret: do not repeat it to the end user or save it anywhere.",
		CompleteArgsLabel:    "pending auth plus the confirmation code",
		Troubleshooting: []agentauth.ConsentGuidanceRow{
			{Symptom: "malformed confirmation code", Meaning: "the code is not in the XXXX-XXXX format and no attempt was consumed", Action: "confirm the code with the end user, then rerun `tollbit auth complete <code>`"},
			{Symptom: "authorization denied", Meaning: "the code was wrong and the challenge is dead", Action: "restart with `tollbit auth login`"},
			{Symptom: "authorization still pending", Meaning: "the end user has not clicked Authorize yet", Action: "wait for the end user to finish, then rerun `tollbit auth complete <code>`"},
			{Symptom: "authorization expired or invalid", Meaning: "the challenge can no longer be completed", Action: "restart with `tollbit auth login`"},
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

	startResp, err := s.authClient.StartAgentConsentCodeAuthorization(ctx, baseToken, auth.ConsentCodeAuthorizationStartRequest{
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
			fmt.Fprintln(stdout, "Open this URL in the end user's browser to continue:")
		} else {
			fmt.Fprintln(stdout, "Opened authorization page in your browser.")
			fmt.Fprintln(stdout, "If it did not open, visit:")
		}
	} else {
		fmt.Fprintln(stdout, "Relay ONLY this URL to the end user. The end user must open it in their browser, sign in, and click Authorize:")
	}
	fmt.Fprintf(stdout, "%s\n\n", startResp.ConsentURL)
	fmt.Fprintln(stdout, "The page then shows a confirmation code in the form XXXX-XXXX. Ask the user to paste it back, then run `tollbit auth complete <code>`.")

	pending := agentauth.PendingConsent{
		Method:           agentauth.ConsentMethodCodeAuthorization,
		ChallengeID:      startResp.ChallengeID,
		AgentIdentity:    identity,
		BaseToken:        baseToken.RawToken,
		CodeVerifier:     codeVerifier,
		UseRefreshTokens: s.useRefreshTokens,
		CreatedAt:        time.Now().UTC(),
		ExpiresAt:        startResp.ExpiresAt,
	}
	return auth.AgentTokenResponse{}, agentauth.AuthorizationPendingError{Pending: pending}
}

func (s ConsentStrategy) CompleteDetached(inv agentauth.Invocation, pending agentauth.PendingConsent, input agentauth.CompleteDetachedInput) (auth.AgentTokenResponse, error) {
	if pending.Method != agentauth.ConsentMethodCodeAuthorization {
		return auth.AgentTokenResponse{}, fmt.Errorf("pending consent method %q is not supported by code-authorization", pending.Method)
	}
	code, ok := normalizeCode(input.Code)
	if !ok {
		return auth.AgentTokenResponse{}, errors.New("malformed confirmation code: expected 8 characters in the form XXXX-XXXX, using letters and digits without 0, O, 1, I, or L. Nothing was sent to TollBit and the attempt was not used up. Confirm the code with the end user, then rerun `tollbit auth complete <code>`.")
	}
	var ua *string
	if pending.AgentIdentity.UserAgent != "" {
		ua = &pending.AgentIdentity.UserAgent
	}
	resp, err := s.authClient.RedeemAgentConsentCodeAuthorization(inv.Context(), agent.Token{RawToken: pending.BaseToken}, auth.ConsentCodeAuthorizationTokenRequest{
		AgentIdentifier: pending.AgentIdentity.Name,
		ChallengeID:     pending.ChallengeID,
		Code:            code,
		CodeVerifier:    pending.CodeVerifier,
		UA:              ua,
		WBA:             pending.AgentIdentity.WBA,
	})
	if err != nil {
		return auth.AgentTokenResponse{}, explainRedeemError(err)
	}
	return resp, nil
}

// normalizeCode returns the code as XXXX-XXXX, or false when it is not in the
// code format. It reports only a verdict so that callers cannot echo the input.
func normalizeCode(input string) (string, bool) {
	code := strings.Map(func(r rune) rune {
		if r == '-' || unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToUpper(r)
	}, input)
	if len(code) != codeLength {
		return "", false
	}
	for _, r := range code {
		if !strings.ContainsRune(codeAlphabet, r) {
			return "", false
		}
	}
	return code[:codeLength/2] + "-" + code[codeLength/2:], true
}

// explainRedeemError says whether the attempt survived, because the next step
// differs: a malformed code can be retried, and anything else needs a new login.
func explainRedeemError(err error) error {
	var problem problemjson.Problem
	if !errors.As(err, &problem) || problem.Code == nil {
		return err
	}
	switch *problem.Code {
	case problemjson.ErrorCodeMalformedCode:
		return fmt.Errorf("%w\nThe attempt was not used up. Confirm the code with the end user, then rerun `tollbit auth complete <code>`.", err)
	case problemjson.ErrorCodeAccessDenied:
		return fmt.Errorf("%w\nThe code was wrong, or an earlier attempt already failed, so this authorization is dead. Run `tollbit auth login` to start over.", err)
	case problemjson.ErrorCodeExpiredToken:
		return fmt.Errorf("%w\nThis authorization expired. Run `tollbit auth login` to start over.", err)
	case problemjson.ErrorCodeInvalidGrant:
		return fmt.Errorf("%w\nThis authorization can no longer be completed. Run `tollbit auth login` to start over.", err)
	default:
		return err
	}
}

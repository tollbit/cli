package redirect

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/agentauth/common"
	"github.com/tollbit/cli/internal/client/auth"
	"github.com/tollbit/cli/internal/cliruntime"
	"github.com/tollbit/cli/internal/oauth/loopback"
	"github.com/tollbit/cli/internal/tokens/agent"
)

type (
	ConsentStrategyConfig struct {
		AuthClient       *auth.Client
		Runtime          *cliruntime.Runtime
		CallbackAddress  string
		AutoOpenBrowser  bool
		Timeout          time.Duration
		UseRefreshTokens bool
	}

	ConsentStrategy struct {
		authClient       *auth.Client
		runtime          *cliruntime.Runtime
		callbackAddress  string
		autoOpenBrowser  bool
		timeout          time.Duration
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
	if strings.TrimSpace(cfg.CallbackAddress) == "" {
		return nil, errors.New("callback address is required")
	}
	if cfg.Timeout < 0 {
		return nil, errors.New("timeout must be non-negative")
	}
	return &ConsentStrategy{
		authClient:       cfg.AuthClient,
		runtime:          cfg.Runtime,
		callbackAddress:  strings.TrimSpace(cfg.CallbackAddress),
		autoOpenBrowser:  cfg.AutoOpenBrowser,
		timeout:          cfg.Timeout,
		useRefreshTokens: cfg.UseRefreshTokens,
	}, nil
}

func (a ConsentStrategy) Method() agentauth.ConsentMethod {
	return agentauth.ConsentMethodRedirect
}

func (a ConsentStrategy) AutoOpensBrowser() bool {
	return a.autoOpenBrowser
}

func (a ConsentStrategy) SupportsOBORetry() bool {
	return true
}

func (a ConsentStrategy) SupportsDetachedCompletion() bool {
	return false
}

func (a ConsentStrategy) Guidance() agentauth.ConsentGuidance {
	return agentauth.ConsentGuidance{
		FlowLabel:            "local browser",
		LoginInstructions:    "For local authorization, run `tollbit auth login` in the same environment as the end user's browser. The CLI opens the browser when configured to do so, waits for the loopback callback, and completes authorization automatically.",
		CompleteInstructions: "No detached completion command is used for the local browser flow; if authorization is interrupted, rerun `tollbit auth login`.",
		CompleteArgsLabel:    "not used (completion is automatic)",
		Troubleshooting: []agentauth.ConsentGuidanceRow{
			{Symptom: "browser did not open", Meaning: "automatic browser launch failed or is disabled", Action: "open the printed URL manually in the local browser"},
			{Symptom: "authorization timed out", Meaning: "the loopback callback did not complete before the timeout", Action: "rerun `tollbit auth login`"},
		},
	}
}

func (a ConsentStrategy) AuthorizeOBO(inv agentauth.Invocation, identity auth.AgentIdentity, baseToken agent.Token) (auth.AgentTokenResponse, error) {
	ctx := inv.Context()

	callback, err := loopback.Start(ctx, a.callbackAddress)
	if err != nil {
		return auth.AgentTokenResponse{}, err
	}
	defer callback.Close()

	codeVerifier, codeChallenge, err := common.GeneratePKCE()
	if err != nil {
		return auth.AgentTokenResponse{}, err
	}
	state, err := common.RandomURLToken(32)
	if err != nil {
		return auth.AgentTokenResponse{}, err
	}
	scope := ""
	if a.useRefreshTokens {
		scope = "offline_access"
	}

	startResp, err := a.authClient.StartAgentConsentRedirect(ctx, baseToken, auth.ConsentRedirectStartRequest{
		RedirectURI:         callback.RedirectURI,
		State:               state,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: common.PKCEChallengeMethod,
		Scope:               scope,
	})
	if err != nil {
		return auth.AgentTokenResponse{}, err
	}
	if strings.TrimSpace(startResp.ConsentURL) == "" {
		return auth.AgentTokenResponse{}, errors.New("auth did not return a consent URL")
	}

	stdout := inv.OutOrStdout()
	fmt.Fprintf(stdout, "Authorize agent: %s\n\n", identity.Name)
	if a.autoOpenBrowser {
		if err := a.runtime.OpenBrowser(startResp.ConsentURL); err != nil {
			fmt.Fprintf(stdout, "Could not open your browser automatically: %v\n", err)
			fmt.Fprintln(stdout, "Open this URL in your browser to continue:")
		} else {
			fmt.Fprintln(stdout, "Opened authorization page in your browser.")
			fmt.Fprintln(stdout, "If it did not open, visit:")
		}
	} else {
		fmt.Fprintln(stdout, "Open this URL in your browser to continue:")
	}
	fmt.Fprintf(stdout, "%s\n\n", startResp.ConsentURL)
	fmt.Fprintf(stdout, "Waiting for authorization at %s\n", callback.RedirectURI)

	waitCtx := ctx
	cancel := func() {}
	timeout := a.timeout
	if timeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	result, err := callback.Wait(waitCtx)
	if err != nil {
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			return auth.AgentTokenResponse{}, fmt.Errorf("authorization timed out; no agent token was saved")
		}
		return auth.AgentTokenResponse{}, err
	}
	if result.Err != nil {
		return auth.AgentTokenResponse{}, result.Err
	}
	if result.State != state {
		return auth.AgentTokenResponse{}, fmt.Errorf("callback state mismatch")
	}
	var ua *string
	if identity.UserAgent != "" {
		ua = &identity.UserAgent
	}

	resp, err := a.authClient.RedeemAgentConsentRedirect(ctx, baseToken, auth.ConsentRedirectTokenRequest{
		AgentIdentifier: identity.Name,
		Code:            result.Code,
		CodeVerifier:    codeVerifier,
		RedirectURI:     callback.RedirectURI,
		UA:              ua,
		WBA:             identity.WBA,
	})
	if err != nil {
		return auth.AgentTokenResponse{}, err
	}
	oboToken := agent.Token{RawToken: resp.Token}
	if err := oboToken.Validate(); err != nil {
		return auth.AgentTokenResponse{}, err
	}
	return resp, nil
}

func (a ConsentStrategy) CompleteDetached(inv agentauth.Invocation, pending agentauth.PendingConsent, input agentauth.CompleteDetachedInput) (auth.AgentTokenResponse, error) {
	return auth.AgentTokenResponse{}, errors.New("redirect consent does not support detached completion")
}

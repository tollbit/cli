package agentauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/tollbit/tollbit-cli/internal/client/auth"
	"github.com/tollbit/tollbit-cli/internal/oauth/loopback"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
)

const (
	pkceChallengeMethod = "S256"
)

type BrowserConsentAuthorizerConfig struct {
	AuthClient      *auth.Client
	CallbackAddress string
	AutoOpenBrowser bool
	Timeout         time.Duration
}

type BrowserConsentAuthorizer struct {
	authClient      *auth.Client
	callbackAddress string
	autoOpenBrowser bool
	timeout         time.Duration
}

func NewBrowserConsentAuthorizer(cfg BrowserConsentAuthorizerConfig) (*BrowserConsentAuthorizer, error) {
	if cfg.AuthClient == nil {
		return nil, errors.New("auth client is required")
	}
	if strings.TrimSpace(cfg.CallbackAddress) == "" {
		return nil, errors.New("callback address is required")
	}
	if cfg.Timeout < 0 {
		return nil, errors.New("timeout must be non-negative")
	}
	return &BrowserConsentAuthorizer{
		authClient:      cfg.AuthClient,
		callbackAddress: strings.TrimSpace(cfg.CallbackAddress),
		autoOpenBrowser: cfg.AutoOpenBrowser,
		timeout:         cfg.Timeout,
	}, nil
}

func (a BrowserConsentAuthorizer) AuthorizeOBO(inv Invocation, identity auth.AgentIdentity, baseToken agent.Token) (agent.Token, error) {
	ctx := inv.Context()

	callback, err := loopback.Start(ctx, a.callbackAddress)
	if err != nil {
		return agent.Token{}, err
	}
	defer callback.Close()

	codeVerifier, codeChallenge, err := generatePKCE()
	if err != nil {
		return agent.Token{}, err
	}
	state, err := randomURLToken(32)
	if err != nil {
		return agent.Token{}, err
	}

	startResp, err := a.authClient.StartAgentConsentRedirect(ctx, baseToken, auth.ConsentRedirectStartRequest{
		RedirectURI:         callback.RedirectURI,
		State:               state,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: pkceChallengeMethod,
	})
	if err != nil {
		return agent.Token{}, err
	}
	if strings.TrimSpace(startResp.ConsentURL) == "" {
		return agent.Token{}, errors.New("auth did not return a consent URL")
	}

	stdout := inv.OutOrStdout()
	fmt.Fprintf(stdout, "Authorize agent: %s\n\n", identity.Name)
	if a.autoOpenBrowser {
		if err := openBrowser(startResp.ConsentURL); err != nil {
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
			return agent.Token{}, fmt.Errorf("authorization timed out; no agent token was saved")
		}
		return agent.Token{}, err
	}
	if result.Err != nil {
		return agent.Token{}, result.Err
	}
	if result.State != state {
		return agent.Token{}, fmt.Errorf("callback state mismatch")
	}

	oboToken, err := a.authClient.RedeemAgentConsentRedirect(ctx, baseToken, auth.ConsentRedirectTokenRequest{
		Code:         result.Code,
		CodeVerifier: codeVerifier,
		RedirectURI:  callback.RedirectURI,
	})
	if err != nil {
		return agent.Token{}, err
	}
	if err := oboToken.Validate(); err != nil {
		return agent.Token{}, err
	}
	return oboToken, nil
}

func generatePKCE() (string, string, error) {
	verifier, err := randomURLToken(48)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func randomURLToken(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func openBrowser(rawURL string) error {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	default:
		return exec.Command("xdg-open", rawURL).Start()
	}
}

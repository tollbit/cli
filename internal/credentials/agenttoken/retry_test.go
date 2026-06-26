package agenttoken

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/tollbit/tollbit-cli/internal/agentauth"
	"github.com/tollbit/tollbit-cli/internal/client/auth"
	"github.com/tollbit/tollbit-cli/internal/errorsx/problemjson"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
)

func TestWithOBORetryReturnsFirstCallSuccess(t *testing.T) {
	baseToken := agent.Token{RawToken: testJWT(t, validClaims())}
	mgr := newRetryTestManager(t, baseToken, nil)
	var calls []agent.Token

	got, err := WithOBORetry(testInvocation{}, mgr, testAgentIdentity(), func(token agent.Token) (string, error) {
		calls = append(calls, token)
		return "ok", nil
	})

	if err != nil {
		t.Fatal(err)
	}
	if got != "ok" {
		t.Fatalf("got %q", got)
	}
	if len(calls) != 1 || calls[0] != baseToken {
		t.Fatalf("expected one base token call, got %#v", calls)
	}
}

func TestWithOBORetryDoesNotRetryNonOBOError(t *testing.T) {
	baseToken := agent.Token{RawToken: testJWT(t, validClaims())}
	mgr := newRetryTestManager(t, baseToken, nil)
	wantErr := errors.New("forbidden for another reason")
	callCount := 0

	got, err := WithOBORetry(testInvocation{}, mgr, testAgentIdentity(), func(token agent.Token) (string, error) {
		callCount++
		return "first", wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if got != "first" {
		t.Fatalf("got %q", got)
	}
	if callCount != 1 {
		t.Fatalf("expected one call, got %d", callCount)
	}
}

func TestWithOBORetryRetriesOBORequiredWithOBOToken(t *testing.T) {
	baseToken := agent.Token{RawToken: testJWT(t, validClaims())}
	oboToken := agent.Token{RawToken: testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", Org: "org-123"}
		return claims
	}))}
	authorizer := &retryOBOAuthorizer{token: oboToken}
	mgr := newRetryTestManager(t, baseToken, authorizer)
	var calls []agent.Token

	got, err := WithOBORetry(testInvocation{}, mgr, testAgentIdentity(), func(token agent.Token) (string, error) {
		calls = append(calls, token)
		if len(calls) == 1 {
			return "", oboRequiredProblem()
		}
		return "ok", nil
	})

	if err != nil {
		t.Fatal(err)
	}
	if got != "ok" {
		t.Fatalf("got %q", got)
	}
	if len(calls) != 2 || calls[0] != baseToken || calls[1] != oboToken {
		t.Fatalf("expected base then OBO token calls, got %#v", calls)
	}
	if authorizer.callCount != 1 || authorizer.baseToken != baseToken {
		t.Fatalf("unexpected authorizer state: %#v", authorizer)
	}
}

func TestWithOBORetryReturnsOBOAuthorizationError(t *testing.T) {
	baseToken := agent.Token{RawToken: testJWT(t, validClaims())}
	wantErr := errors.New("consent failed")
	authorizer := &retryOBOAuthorizer{err: wantErr}
	mgr := newRetryTestManager(t, baseToken, authorizer)
	callCount := 0

	_, err := WithOBORetry(testInvocation{}, mgr, testAgentIdentity(), func(token agent.Token) (string, error) {
		callCount++
		return "", oboRequiredProblem()
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if callCount != 1 {
		t.Fatalf("expected no retry after authorization failure, got %d calls", callCount)
	}
}

func TestWithOBORetryReturnsRetryError(t *testing.T) {
	baseToken := agent.Token{RawToken: testJWT(t, validClaims())}
	oboToken := agent.Token{RawToken: testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", Org: "org-123"}
		return claims
	}))}
	mgr := newRetryTestManager(t, baseToken, &retryOBOAuthorizer{token: oboToken})
	wantErr := errors.New("retry failed")
	callCount := 0

	_, err := WithOBORetry(testInvocation{}, mgr, testAgentIdentity(), func(token agent.Token) (string, error) {
		callCount++
		if callCount == 1 {
			return "", oboRequiredProblem()
		}
		return "", wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if callCount != 2 {
		t.Fatalf("expected retry call, got %d calls", callCount)
	}
}

func newRetryTestManager(t *testing.T, token agent.Token, authorizer agentauth.OBOAuthorizer) *CredentialManager {
	t.Helper()
	return newTestManagerWithConfig(t, t.TempDir(), CredentialManagerConfig{OBOAuthorizer: authorizer}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token.RawToken})
	}))
}

type retryOBOAuthorizer struct {
	token     agent.Token
	err       error
	baseToken agent.Token
	callCount int
}

func (a *retryOBOAuthorizer) AuthorizeOBO(inv agentauth.Invocation, identity auth.AgentIdentity, baseToken agent.Token) (agent.Token, error) {
	a.callCount++
	a.baseToken = baseToken
	if a.err != nil {
		return agent.Token{}, a.err
	}
	return a.token, nil
}

func oboRequiredProblem() problemjson.Problem {
	code := problemjson.ErrorCodeOboRequired
	detail := "OBO required"
	return problemjson.Problem{
		Title:  "Forbidden",
		Status: http.StatusForbidden,
		Detail: &detail,
		Code:   &code,
	}
}

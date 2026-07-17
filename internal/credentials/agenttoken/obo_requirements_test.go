package agenttoken

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/client/auth"
	"github.com/tollbit/cli/internal/errorsx/problemjson"
	"github.com/tollbit/cli/internal/tokens/agent"
)

func TestWithOBORetryReturnsFirstCallSuccess(t *testing.T) {
	mgr := newTestManager(t, t.TempDir(), testMintHandler(t, testJWT(t, validClaims())))

	var calls int
	got, err := WithOBORetry(testInvocation{}, mgr, testAgentIdentity(), func(token agent.Token) (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "ok" || calls != 1 {
		t.Fatalf("unexpected result got=%q calls=%d", got, calls)
	}
}

func TestWithOBORetryDoesNotRetryNonOBOError(t *testing.T) {
	mgr := newTestManager(t, t.TempDir(), testMintHandler(t, testJWT(t, validClaims())))
	wantErr := errors.New("boom")

	var calls int
	got, err := WithOBORetry(testInvocation{}, mgr, testAgentIdentity(), func(token agent.Token) (string, error) {
		calls++
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wantErr, got %v", err)
	}
	if got != "" || calls != 1 {
		t.Fatalf("unexpected result got=%q calls=%d", got, calls)
	}
}

func TestWithOBORetryRetriesOBORequiredWithOBOToken(t *testing.T) {
	baseToken := testJWT(t, validClaims())
	oboToken := testJWT(t, validClaims(func(claims agent.Claims) agent.Claims {
		claims.OBO = &agent.OBOClaims{Ver: 1, Source: "consent", Org: "org-123"}
		return claims
	}))
	authorizer := &retryOBOAuthorizer{token: oboToken}
	mgr := newTestManagerWithConfig(t, t.TempDir(), CredentialManagerConfig{OBOAuthorizer: authorizer}, testMintHandler(t, baseToken))

	var calls []string
	got, err := WithOBORetry(testInvocation{}, mgr, testAgentIdentity(), func(token agent.Token) (string, error) {
		calls = append(calls, token.RawToken)
		if len(calls) == 1 {
			code := problemjson.ErrorCodeOboRequired
			return "", problemjson.Problem{Code: &code}
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "ok" {
		t.Fatalf("expected ok, got %q", got)
	}
	if len(calls) != 2 || calls[0] != baseToken || calls[1] != oboToken {
		t.Fatalf("expected base then OBO token calls, got %#v", calls)
	}
}

func TestWithOBORetryReturnsOBOAuthorizationError(t *testing.T) {
	baseToken := testJWT(t, validClaims())
	wantErr := errors.New("consent failed")
	authorizer := &retryOBOAuthorizer{err: wantErr}
	mgr := newTestManagerWithConfig(t, t.TempDir(), CredentialManagerConfig{OBOAuthorizer: authorizer}, testMintHandler(t, baseToken))

	_, err := WithOBORetry(testInvocation{}, mgr, testAgentIdentity(), func(token agent.Token) (string, error) {
		code := problemjson.ErrorCodeOboRequired
		return "", problemjson.Problem{Code: &code}
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wantErr, got %v", err)
	}
}

type retryOBOAuthorizer struct {
	token string
	err   error
}

func (a *retryOBOAuthorizer) AuthorizeOBO(inv agentauth.Invocation, identity auth.AgentIdentity, baseToken agent.Token) (auth.AgentTokenResponse, error) {
	if a.err != nil {
		return auth.AgentTokenResponse{}, a.err
	}
	return auth.AgentTokenResponse{Token: a.token}, nil
}

func testMintHandler(t *testing.T, token string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(auth.AgentTokenResponse{Token: token})
	}
}

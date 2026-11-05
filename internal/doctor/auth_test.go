package doctor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tollbit/cli/internal/credentials/agenttoken"
	"github.com/tollbit/cli/internal/tokens/agent"
)

type fakeStore struct {
	token    agent.Token
	stored   bool
	tokenErr error
	refresh  agenttoken.RefreshTokenStatus
}

func (f fakeStore) CurrentAgentToken(context.Context) (agent.Token, bool, error) {
	return f.token, f.stored, f.tokenErr
}

func (f fakeStore) RefreshTokenStatus(context.Context) agenttoken.RefreshTokenStatus {
	return f.refresh
}

func testToken(t *testing.T, expiresAt time.Time) agent.Token {
	t.Helper()
	segment := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	raw := segment(map[string]any{"alg": "none"}) + "." +
		segment(map[string]any{"tbt": "agent-token", "exp": expiresAt.Unix()}) + ".c2ln"
	return agent.Token{RawToken: raw}
}

func TestAuthCheck(t *testing.T) {
	valid := testToken(t, time.Date(2099, 1, 2, 3, 4, 0, 0, time.UTC))
	expired := testToken(t, time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC))
	malformed := agent.Token{RawToken: "not-a-jwt"}
	for _, tt := range []struct {
		name  string
		store fakeStore
		want  string
		hint  bool
	}{
		{"logged out", fakeStore{}, "not logged in", true},
		{"valid", fakeStore{token: valid, stored: true}, "token stored, expires 2099-01-02 03:04 UTC", false},
		{"expired with refresh", fakeStore{token: expired, stored: true, tokenErr: errors.New("agent token expired"), refresh: agenttoken.RefreshTokenStatus{Present: true}},
			"token stored, expired 2026-01-02 03:04 UTC; refresh token stored", false},
		{"malformed", fakeStore{token: malformed, stored: true, tokenErr: errors.New("couldn't get agent token claims")},
			"token stored but invalid (couldn't get agent token claims)", true},
		{"refresh expired", fakeStore{token: expired, stored: true, tokenErr: errors.New("agent token expired"), refresh: agenttoken.RefreshTokenStatus{Present: true, Expired: true}},
			"token stored, expired 2026-01-02 03:04 UTC; refresh token expired", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := authCheck(context.Background(), tt.store)
			if c.Status != StatusInfo || c.Detail != tt.want || (c.Hint != "") != tt.hint {
				t.Fatalf("got %+v, want detail %q, hint %t", c, tt.want, tt.hint)
			}
			if strings.Contains(c.Detail, tt.store.token.RawToken) && tt.store.token.RawToken != "" {
				t.Fatalf("token leaked into %q", c.Detail)
			}
		})
	}
}

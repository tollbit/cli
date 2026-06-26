package agent

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenValid(t *testing.T) {
	token := Token{RawToken: testJWT(t, testClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://oauth.tollbit.com",
			Audience:  jwt.ClaimStrings{"tollbit.com"},
			Subject:   "agent-test",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			ID:        "agtok_test",
		},
		TBT: "agent-token",
		UA:  "agent-test/0.1",
		WBA: &WBAClaims{Ver: 1, Dir: "https://example.com/.well-known/http-message-signatures-directory", Req: false},
	})}

	if err := token.Validate(); err != nil {
		t.Fatalf("expected token to validate: %v", err)
	}
	if !token.Valid() {
		t.Fatal("expected token to be valid")
	}

	claims, err := token.Claims()
	if err != nil {
		t.Fatal(err)
	}
	if claims.TBT != "agent-token" || claims.Subject != "agent-test" || claims.UA != "agent-test/0.1" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if claims.WBA == nil || claims.WBA.Ver != 1 {
		t.Fatalf("unexpected wba claims: %#v", claims.WBA)
	}
	if _, ok := token.ExpiresAt(); !ok {
		t.Fatal("expected expires at")
	}
	if _, ok := token.NotBefore(); !ok {
		t.Fatal("expected not before")
	}
}

func TestTokenClaimsIncludesOnBehalfOf(t *testing.T) {
	token := Token{RawToken: testJWT(t, validTestClaims(func(claims testClaims) testClaims {
		claims.OBO = &OBOClaims{Ver: 1, Source: "consent", User: "usr_123", Org: "org_456"}
		return claims
	}))}

	claims, err := token.Claims()
	if err != nil {
		t.Fatal(err)
	}
	if claims.OBO == nil || claims.OBO.Source != "consent" || claims.OBO.User != "usr_123" || claims.OBO.Org != "org_456" {
		t.Fatalf("unexpected obo claims: %#v", claims.OBO)
	}
}

func TestTokenValidWithOnlyLocalCacheClaims(t *testing.T) {
	token := Token{RawToken: testJWT(t, testClaims{
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		TBT:              "agent-token",
	})}

	if err := token.Validate(); err != nil {
		t.Fatalf("expected token to validate: %v", err)
	}
}

func TestTokenValidateErrors(t *testing.T) {
	tests := []struct {
		name    string
		token   Token
		wantErr string
	}{
		{
			name:    "empty token",
			token:   Token{},
			wantErr: "empty",
		},
		{
			name:    "malformed token",
			token:   Token{RawToken: "not-a-jwt"},
			wantErr: "parse agent token claims",
		},
		{
			name: "expired token",
			token: Token{RawToken: testJWT(t, validTestClaims(func(claims testClaims) testClaims {
				claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
				return claims
			}))},
			wantErr: "expired",
		},
		{
			name: "future nbf",
			token: Token{RawToken: testJWT(t, validTestClaims(func(claims testClaims) testClaims {
				claims.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Hour))
				return claims
			}))},
			wantErr: "not valid yet",
		},
		{
			name: "wrong tbt",
			token: Token{RawToken: testJWT(t, validTestClaims(func(claims testClaims) testClaims {
				claims.TBT = "oauth"
				return claims
			}))},
			wantErr: "invalid tbt",
		},
		{
			name: "missing exp",
			token: Token{RawToken: testJWT(t, validTestClaims(func(claims testClaims) testClaims {
				claims.ExpiresAt = nil
				return claims
			}))},
			wantErr: "missing exp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.token.Validate()
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestTokenExpired(t *testing.T) {
	expired := Token{RawToken: testJWT(t, validTestClaims(func(claims testClaims) testClaims {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
		return claims
	}))}
	valid := Token{RawToken: testJWT(t, validTestClaims())}

	if !expired.Expired() {
		t.Fatal("expected expired token to be expired")
	}
	if valid.Expired() {
		t.Fatal("expected valid token not to be expired")
	}
}

type testClaims Claims

func validTestClaims(mutators ...func(testClaims) testClaims) testClaims {
	claims := testClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://oauth.tollbit.com",
			Audience:  jwt.ClaimStrings{"tollbit.com"},
			Subject:   "agent-test",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			ID:        "agtok_test",
		},
		TBT: "agent-token",
	}
	for _, mutate := range mutators {
		claims = mutate(claims)
	}
	return claims
}

func testJWT(t *testing.T, claims testClaims) string {
	t.Helper()
	header := map[string]any{"alg": "none"}
	encodedHeader := encodeJSONSegment(t, header)
	encodedClaims := encodeJSONSegment(t, claims)
	return encodedHeader + "." + encodedClaims + "." + base64.RawURLEncoding.EncodeToString([]byte("signature"))
}

func encodeJSONSegment(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

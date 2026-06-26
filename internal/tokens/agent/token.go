package agent

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const tokenExpirySkew = 30 * time.Second

type (
	Token struct {
		RawToken string
	}

	Claims struct {
		jwt.RegisteredClaims
		TBT string     `json:"tbt"`
		UA  string     `json:"ua,omitempty"`
		WBA *WBAClaims `json:"wba,omitempty"`
		OBO *OBOClaims `json:"obo,omitempty"`
	}

	WBAClaims struct {
		Ver int32  `json:"ver"`
		Dir string `json:"dir"`
		Req bool   `json:"req"`
	}

	OBOClaims struct {
		Ver    int32  `json:"ver"`
		Source string `json:"src"`
		User   string `json:"usr,omitempty"`
		Org    string `json:"org,omitempty"`
	}
)

func (t Token) Claims() (Claims, error) {
	if strings.TrimSpace(t.RawToken) == "" {
		return Claims{}, errors.New("agent token is empty")
	}

	var claims Claims
	parser := jwt.NewParser()
	if _, _, err := parser.ParseUnverified(t.RawToken, &claims); err != nil {
		return Claims{}, fmt.Errorf("parse agent token claims: %w", err)
	}
	return claims, nil
}

func (t Token) Validate() error {
	claims, err := t.Claims()
	if err != nil {
		return NewInvalidTokenErrf("couldn't get agent token claims: %w", err)
	}

	if claims.TBT != "agent-token" {
		return NewInvalidTokenErr("agent token has invalid tbt claim")
	}
	if claims.ExpiresAt == nil {
		return NewInvalidTokenErr("agent token missing exp claim")
	}

	now := time.Now().UTC()
	if claims.NotBefore != nil && now.Before(claims.NotBefore.Time.UTC()) {
		return NewInvalidTokenErr("agent token is not valid yet")
	}
	if !now.Add(tokenExpirySkew).Before(claims.ExpiresAt.Time.UTC()) {
		return NewInvalidTokenErr("agent token expired")
	}

	return nil
}

func (t Token) Valid() bool {
	return t.Validate() == nil
}

func (t Token) Expired() bool {
	expiresAt, ok := t.ExpiresAt()
	if !ok {
		return true
	}
	return !time.Now().UTC().Add(tokenExpirySkew).Before(expiresAt)
}

func (t Token) ExpiresAt() (time.Time, bool) {
	claims, err := t.Claims()
	if err != nil || claims.ExpiresAt == nil {
		return time.Time{}, false
	}
	return claims.ExpiresAt.Time.UTC(), true
}

func (t Token) NotBefore() (time.Time, bool) {
	claims, err := t.Claims()
	if err != nil || claims.NotBefore == nil {
		return time.Time{}, false
	}
	return claims.NotBefore.Time.UTC(), true
}

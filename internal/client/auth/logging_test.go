package auth

import (
	"strings"
	"testing"
)

func TestRedactLogBodyRedactsTokenField(t *testing.T) {
	got := redactLogBody([]byte(`{"token":"eyJhbGciOiJSUzI1NiJ9.abcdef","refresh_token":"agrt_secret_value","expires_in":3600}`))
	if got == "" {
		t.Fatal("expected redacted body")
	}
	if strings.Contains(got, "eyJhbGciOi") {
		t.Fatalf("expected token redacted, got %q", got)
	}
	if strings.Contains(got, "agrt_secret_value") {
		t.Fatalf("expected refresh token redacted, got %q", got)
	}
	if !strings.Contains(got, "expires_in") {
		t.Fatalf("expected other fields preserved, got %q", got)
	}
}

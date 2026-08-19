package logging

import (
	"strings"
	"testing"
)

func TestHTTPBodyRedactorRedactsConfiguredFields(t *testing.T) {
	redactor := NewHTTPBodyRedactor(HTTPBodyRedactorConfig{
		JSONFields: map[string]JSONFieldRedactor{
			"token":         AbbreviateJSONSecret(6, 4),
			"refresh_token": AbbreviateJSONSecret(6, 4),
		},
	})

	got := redactor.Redact("/agent/v1/tokens/identity", []byte(`{"token":"eyJhbGciOiJSUzI1NiJ9.abcdef","refresh_token":"agrt_secret_value","expires_in":3600}`))
	if strings.Contains(got, "eyJhbGciOi") || strings.Contains(got, "agrt_secret_value") {
		t.Fatalf("expected secrets redacted, got %q", got)
	}
	if !strings.Contains(got, "expires_in") {
		t.Fatalf("expected other fields preserved, got %q", got)
	}
}

func TestHTTPBodyRedactorRedactsConfiguredPath(t *testing.T) {
	redactor := NewHTTPBodyRedactor(HTTPBodyRedactorConfig{
		FullyRedactedPaths: []string{"/agent/v1/whoami"},
	})

	got := redactor.Redact("/agent/v1/whoami", []byte(`{"primary_email":"user@example.com"}`))
	if got != RedactedValue {
		t.Fatalf("expected fully redacted body, got %q", got)
	}
}

func TestHTTPBodyRedactorFailsClosedForInvalidJSON(t *testing.T) {
	redactor := NewHTTPBodyRedactor(HTTPBodyRedactorConfig{
		JSONFields: map[string]JSONFieldRedactor{"token": RedactJSONField},
	})

	got := redactor.Redact("/agent/v1/tokens/identity", []byte(`{"token":"secret"`))
	if got != RedactedValue {
		t.Fatalf("expected invalid JSON to be fully redacted, got %q", got)
	}
}

func TestHTTPBodyRedactorRedactsNonStringSecrets(t *testing.T) {
	redactor := NewHTTPBodyRedactor(HTTPBodyRedactorConfig{
		JSONFields: map[string]JSONFieldRedactor{"token": AbbreviateJSONSecret(6, 4)},
	})

	got := redactor.Redact("/agent/v1/tokens/identity", []byte(`{"token":{"nested":"secret"}}`))
	if strings.Contains(got, "nested") || !strings.Contains(got, RedactedValue) {
		t.Fatalf("expected non-string secret to be fully redacted, got %q", got)
	}
}

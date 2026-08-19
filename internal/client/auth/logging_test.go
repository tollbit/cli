package auth

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestLogResponseSuppressesWhoAmIBody(t *testing.T) {
	var logs bytes.Buffer
	logger := zerolog.New(&logs).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())
	logResponse(ctx, "GET", "https://oauth.tollbit.com/agent/v1/whoami", nil, 200, "200 OK", []byte(`{"agent_identifier":"agent-test","organization_name":"Example Org","primary_email":"user@example.com"}`))
	if strings.Contains(logs.String(), "user@example.com") {
		t.Fatalf("expected whoami email suppressed, got %q", logs.String())
	}
	if !strings.Contains(logs.String(), "[REDACTED]") {
		t.Fatalf("expected redacted response body, got %q", logs.String())
	}
}

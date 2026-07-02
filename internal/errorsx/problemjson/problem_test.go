package problemjson

import (
	"encoding/json"
	"testing"
)

func TestParseStandardProblemJSON(t *testing.T) {
	problem, err := Parse([]byte(`{
		"type": "about:blank",
		"title": "Forbidden",
		"status": 403,
		"detail": "access denied",
		"instance": "/agent-fns/v1/payment-tokens",
		"requestId": "req-123"
	}`))
	if err != nil {
		t.Fatal(err)
	}

	if problem.Type != "about:blank" {
		t.Fatalf("type = %q", problem.Type)
	}
	if problem.Title != "Forbidden" {
		t.Fatalf("title = %q", problem.Title)
	}
	if problem.Status != 403 {
		t.Fatalf("status = %d", problem.Status)
	}
	if problem.Detail == nil || *problem.Detail != "access denied" {
		t.Fatalf("detail = %#v", problem.Detail)
	}
	if problem.Instance == nil || *problem.Instance != "/agent-fns/v1/payment-tokens" {
		t.Fatalf("instance = %#v", problem.Instance)
	}
	if problem.RequestID == nil || *problem.RequestID != "req-123" {
		t.Fatalf("requestId = %#v", problem.RequestID)
	}
	if problem.DetailOrTitle() != "access denied" {
		t.Fatalf("detail or title = %q", problem.DetailOrTitle())
	}
}

func TestParseOBORequiredProblemJSON(t *testing.T) {
	problem, err := Parse([]byte(`{
		"type": "about:blank",
		"title": "Forbidden",
		"status": 403,
		"detail": "on-behalf-of org claim is required",
		"code": "obo_required",
		"required": {
			"obo": {
				"org": true,
				"user": false
			}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	if problem.Code == nil || *problem.Code != ErrorCodeOboRequired {
		t.Fatalf("code = %#v", problem.Code)
	}
	if !problem.IsOBORequired() {
		t.Fatal("expected OBO required")
	}
	required, ok := problem.RequiredOBO()
	if !ok {
		t.Fatal("expected OBO requirement")
	}
	if !required.Org || required.User {
		t.Fatalf("unexpected OBO requirement: %#v", required)
	}
}

func TestParseUserAgentNotRegisteredProblemJSON(t *testing.T) {
	code := ErrorCodeUserAgentNotRegistered
	problem, err := Parse([]byte(`{
		"type": "about:blank",
		"title": "Bad Request",
		"status": 400,
		"detail": "user agent bad-agent is not registered",
		"code": "user_agent_not_registered"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if problem.Code == nil || *problem.Code != code {
		t.Fatalf("code = %#v", problem.Code)
	}
	if !problem.IsUserAgentNotRegistered() {
		t.Fatal("expected user agent not registered")
	}
}

func TestParsePreservesUnknownCode(t *testing.T) {
	problem, err := Parse([]byte(`{
		"type": "about:blank",
		"title": "Bad Request",
		"status": 400,
		"code": "new_server_code"
	}`))
	if err != nil {
		t.Fatal(err)
	}

	if problem.Code == nil || *problem.Code != ErrorCode("new_server_code") {
		t.Fatalf("code = %#v", problem.Code)
	}
	if problem.IsOBORequired() {
		t.Fatal("unknown code should not be OBO required")
	}
}

func TestParsePreservesAdditionalProperties(t *testing.T) {
	problem, err := Parse([]byte(`{
		"type": "about:blank",
		"title": "Forbidden",
		"status": 403,
		"foo": {"bar": true}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := problem.AdditionalProperties["type"]; ok {
		t.Fatal("known field type should not be additional")
	}
	raw, ok := problem.AdditionalProperties["foo"]
	if !ok {
		t.Fatal("expected foo additional property")
	}
	var value struct {
		Bar bool `json:"bar"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if !value.Bar {
		t.Fatalf("unexpected additional property: %#v", value)
	}
}

func TestParseDetailOrTitleFallsBackToTitle(t *testing.T) {
	problem, err := Parse([]byte(`{
		"type": "about:blank",
		"title": "Forbidden",
		"status": 403
	}`))
	if err != nil {
		t.Fatal(err)
	}

	if problem.DetailOrTitle() != "Forbidden" {
		t.Fatalf("detail or title = %q", problem.DetailOrTitle())
	}
}

func TestProblemError(t *testing.T) {
	requestID := "req-123"
	detail := "access denied"
	problem := Problem{Status: 403, Title: "Forbidden", Detail: &detail, RequestID: &requestID}

	if got := problem.Error(); got != "403 Forbidden: access denied (requestId: req-123)" {
		t.Fatalf("error = %q", got)
	}
}

func TestParseInvalidBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty", body: "  "},
		{name: "invalid json", body: "{"},
		{name: "not problem json", body: `{"foo":"bar"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse([]byte(tt.body)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

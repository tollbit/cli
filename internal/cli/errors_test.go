package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tollbit/cli/internal/errorsx"
	"github.com/tollbit/cli/internal/errorsx/problemjson"
	"github.com/tollbit/cli/internal/installmethod"
)

func updateRequiredProblem() problemjson.Problem {
	code := problemjson.ErrorCodeCLIUpdateRequired
	detail := "TollBit CLI version 0.0.1 is below the minimum supported version 0.1.0."
	return problemjson.Problem{
		Title:  "Upgrade Required",
		Status: 426,
		Detail: &detail,
		Code:   &code,
		AdditionalProperties: map[string]json.RawMessage{
			"minimumVersion": json.RawMessage(`"0.1.0"`),
			"latestVersion":  json.RawMessage(`"0.2.0"`),
		},
	}
}

func TestRenderErrorUpdateRequired(t *testing.T) {
	t.Setenv(installmethod.EnvVar, "npm")

	got := RenderError(updateRequiredProblem())
	want := installmethod.RequiredInstructions(installmethod.MethodNPM, "0.1.0", "0.2.0")
	if got != want {
		t.Fatalf("RenderError = %q, want %q", got, want)
	}
	if !strings.Contains(got, "npm update -g @tollbit/tollbit-cli") {
		t.Fatalf("RenderError %q should carry the npm update command", got)
	}
}

func TestRenderErrorUpdateRequiredWrapped(t *testing.T) {
	t.Setenv(installmethod.EnvVar, "installer")

	wrapped := RuntimeError(fmt.Errorf("search: %w", updateRequiredProblem()))
	got := RenderError(wrapped)
	if !strings.Contains(got, "install.sh") {
		t.Fatalf("RenderError on wrapped problem = %q, want installer instructions", got)
	}
}

func TestRenderErrorPassthrough(t *testing.T) {
	err := errors.New("plain failure")
	if got := RenderError(err); got != "plain failure" {
		t.Fatalf("RenderError = %q, want %q", got, "plain failure")
	}

	problem := problemjson.Problem{Title: "Bad Request", Status: 400}
	if got := RenderError(problem); got != problem.Error() {
		t.Fatalf("RenderError = %q, want %q", got, problem.Error())
	}
}

func entitlementRequiredProblem(t *testing.T, extra string) problemjson.Problem {
	t.Helper()
	problem, err := problemjson.Parse([]byte(`{
		"type": "about:blank",
		"title": "Forbidden",
		"status": 403,
		"requestId": "req-123",
		"code": "entitlement_required"` + extra + `
	}`))
	if err != nil {
		t.Fatal(err)
	}
	return problem
}

func TestRenderErrorEntitlementRequiredWithTermsURL(t *testing.T) {
	problem := entitlementRequiredProblem(t, `,
		"detail": "Developer terms of service not accepted.",
		"terms_url": "https://app.tollbit.com/onboard?flow=developer"`)

	got := RenderError(RuntimeError(problem))
	want := "403 Forbidden: Developer terms of service not accepted. (requestId: req-123)\n" +
		"Relay ONLY this URL to the end user. The end user must open it in their browser, sign in, and accept the terms:\n" +
		"https://app.tollbit.com/onboard?flow=developer\n" +
		"\n" +
		"After the end user accepts the terms, re-run the command."
	if got != want {
		t.Fatalf("RenderError = %q, want %q", got, want)
	}
	if ExitCode(RuntimeError(problem)) != 1 {
		t.Fatalf("ExitCode = %d, want 1", ExitCode(RuntimeError(problem)))
	}
}

func TestRenderErrorEntitlementRequiredWithoutTermsURL(t *testing.T) {
	problem := entitlementRequiredProblem(t, `,
		"detail": "Your organization does not have access to this feature. Contact TollBit to enable it."`)

	got := RenderError(problem)
	want := "403 Forbidden: Your organization does not have access to this feature. Contact TollBit to enable it. (requestId: req-123)"
	if got != want {
		t.Fatalf("RenderError = %q, want %q", got, want)
	}
}

func TestRenderErrorEntitlementRequiredWithoutDetail(t *testing.T) {
	problem := entitlementRequiredProblem(t, "")

	got := RenderError(problem)
	want := "403 Forbidden: Forbidden (requestId: req-123)\n" +
		"Contact TollBit to enable it for your organization."
	if got != want {
		t.Fatalf("RenderError = %q, want %q", got, want)
	}
}

func TestRenderErrorEntitlementRequiredIgnoresEntitlementField(t *testing.T) {
	problem := entitlementRequiredProblem(t, `,
		"detail": "Your organization does not have access to this feature. Contact TollBit to enable it.",
		"entitlement": "analytics:sql_api"`)

	got := RenderError(problem)
	want := "403 Forbidden: Your organization does not have access to this feature. Contact TollBit to enable it. (requestId: req-123)"
	if got != want {
		t.Fatalf("RenderError = %q, want %q", got, want)
	}
}

func TestRenderErrorEntitlementRequiredWrapped(t *testing.T) {
	problem := entitlementRequiredProblem(t, `,
		"detail": "Developer terms of service not accepted.",
		"terms_url": "https://app.tollbit.com/onboard?flow=developer"`)

	got := RenderError(RuntimeError(fmt.Errorf("error fetching rates: %w", problem)))
	if !strings.HasPrefix(got, "error fetching rates: 403 Forbidden: Developer terms of service not accepted. (requestId: req-123)\n") {
		t.Fatalf("RenderError on wrapped problem = %q, want the original error line first", got)
	}
	if !strings.Contains(got, "\nhttps://app.tollbit.com/onboard?flow=developer\n") {
		t.Fatalf("RenderError on wrapped problem = %q, want the terms URL", got)
	}
}

func TestRenderErrorPlainTextForbiddenPassthrough(t *testing.T) {
	err := errorsx.GenericHTTPError{
		Status:     "403 Forbidden",
		StatusCode: 403,
		Body:       "Terms of service not accepted for audiences: [developers]",
	}
	if got := RenderError(RuntimeError(err)); got != err.Error() {
		t.Fatalf("RenderError = %q, want %q", got, err.Error())
	}
}

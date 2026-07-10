package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tollbit/tollbit-cli/internal/errorsx/problemjson"
	"github.com/tollbit/tollbit-cli/internal/installmethod"
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
	if !strings.Contains(got, "npm update -g @tollbit/cli") {
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

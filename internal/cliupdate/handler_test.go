package cliupdate

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/launchdarkly/go-semver"
	"github.com/tollbit/cli/internal/client/versionpolicy"
	"github.com/tollbit/cli/internal/installmethod"
	"github.com/tollbit/cli/internal/version"
)

const versionBelowCurrent = "0.0.0"

// versionAbove returns a version newer than this binary.
func versionAbove(t *testing.T) string {
	t.Helper()
	current, err := semver.Parse(version.String())
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%d.%d.0", current.GetMajor(), current.GetMinor()+1)
}

func newTestHandler(t *testing.T) (*Handler, *bytes.Buffer) {
	t.Helper()
	t.Setenv(installmethod.EnvVar, string(installmethod.MethodNPM))
	out := new(bytes.Buffer)
	return NewHandler(out), out
}

func TestHandleVersionPolicyBelowMinimumReturnsRequiredError(t *testing.T) {
	h, out := newTestHandler(t)
	minimum := versionAbove(t)

	err := h.HandleVersionPolicy(versionpolicy.Policy{Minimum: minimum, Latest: minimum})

	var required RequiredError
	if !errors.As(err, &required) {
		t.Fatalf("expected RequiredError, got %v", err)
	}
	if want := installmethod.RequiredInstructions(installmethod.MethodNPM, minimum, minimum); err.Error() != want {
		t.Fatalf("expected message %q, got %q", want, err.Error())
	}
	if out.Len() != 0 {
		t.Fatalf("expected no notice, got %q", out.String())
	}
}

func TestHandleVersionPolicyAtOrAboveMinimumPasses(t *testing.T) {
	h, out := newTestHandler(t)

	if err := h.HandleVersionPolicy(versionpolicy.Policy{Minimum: versionBelowCurrent, Latest: version.String()}); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no notice, got %q", out.String())
	}
}

func TestHandleVersionPolicyBelowLatestPrintsNoticeOnce(t *testing.T) {
	h, out := newTestHandler(t)
	latest := versionAbove(t)

	if err := h.HandleVersionPolicy(versionpolicy.Policy{Latest: latest}); err != nil {
		t.Fatal(err)
	}
	h.HandleVersionUpgradeNotice(latest)

	want := installmethod.UpdateInstructions(installmethod.MethodNPM, latest) + "\n"
	if out.String() != want {
		t.Fatalf("expected the notice once as %q, got %q", want, out.String())
	}
}

func TestHandleVersionPolicyIgnoresUnparsableBounds(t *testing.T) {
	h, out := newTestHandler(t)

	if err := h.HandleVersionPolicy(versionpolicy.Policy{Minimum: "not-a-version", Latest: "also-not"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "" {
		t.Fatalf("expected no notice, got %q", out.String())
	}
}

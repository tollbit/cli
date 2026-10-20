//go:build windows

package cliupdate

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestCheckNPMUpdateOnWindowsReportsAndStops(t *testing.T) {
	env, npm := newNPMInstallEnv(t, false)

	err := env.check(t)

	var required RequiredError
	if !errors.As(err, &required) {
		t.Fatalf("expected RequiredError, got %v", err)
	}
	if !strings.Contains(err.Error(), "npm update -g @tollbit/tollbit-cli") {
		t.Fatalf("expected the npm instruction, got %q", err.Error())
	}
	want := "Updating the TollBit CLI from 0.1.0 to 0.3.0...\nCould not update the TollBit CLI: npm cannot replace the running binary on Windows\n"
	if env.out.String() != want {
		t.Fatalf("expected stderr %q, got %q", want, env.out.String())
	}
	if calls := npm.NPMCalls(t); slices.Contains(calls, "install -g @tollbit/tollbit-cli@"+latestVersion) {
		t.Fatalf("expected no npm install, got %q", calls)
	}
	env.assertBinary(t, runningVersion)
}

package installmethod

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		raw  string
		want Method
	}{
		{"npm", MethodNPM},
		{" NPM \n", MethodNPM},
		{"installer", MethodInstaller},
		{"Installer", MethodInstaller},
		{"", MethodUnknown},
		{"homebrew", MethodUnknown},
	}
	for _, tc := range testCases {
		if got := parse(tc.raw); got != tc.want {
			t.Errorf("parse(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestDetectEnvVarWins(t *testing.T) {
	t.Setenv(EnvVar, "npm")
	if got := Detect(); got != MethodNPM {
		t.Fatalf("Detect() = %q, want npm", got)
	}
}

func TestDetectUnknownWithoutSignals(t *testing.T) {
	t.Setenv(EnvVar, "")
	// Test binaries run from a temp build dir with no marker file and no
	// node_modules in the path, so detection should fall through to unknown.
	if got := Detect(); got != MethodUnknown {
		t.Fatalf("Detect() = %q, want unknown", got)
	}
}

func TestUpdateInstructions(t *testing.T) {
	testCases := []struct {
		method   Method
		latest   string
		contains []string
	}{
		{MethodNPM, "0.2.0", []string{"(0.2.0)", "npm update -g @tollbit/tollbit-cli"}},
		{MethodInstaller, "0.2.0", []string{"(0.2.0)", "install.sh", "--force"}},
		{MethodUnknown, "", []string{"github.com/tollbit/cli"}},
	}
	for _, tc := range testCases {
		got := UpdateInstructions(tc.method, tc.latest)
		for _, want := range tc.contains {
			if !strings.Contains(got, want) {
				t.Errorf("UpdateInstructions(%q, %q) = %q, missing %q", tc.method, tc.latest, got, want)
			}
		}
	}
}

func TestRequiredInstructions(t *testing.T) {
	testCases := []struct {
		method   Method
		minimum  string
		latest   string
		contains []string
	}{
		{MethodNPM, "0.1.0", "0.2.0", []string{"minimum: 0.1.0", "version 0.2.0", "npm update -g @tollbit/tollbit-cli"}},
		{MethodInstaller, "0.1.0", "", []string{"minimum: 0.1.0", "the latest version", "install.sh"}},
		{MethodUnknown, "", "", []string{"no longer supported", "github.com/tollbit/cli"}},
	}
	for _, tc := range testCases {
		got := RequiredInstructions(tc.method, tc.minimum, tc.latest)
		for _, want := range tc.contains {
			if !strings.Contains(got, want) {
				t.Errorf("RequiredInstructions(%q, %q, %q) = %q, missing %q", tc.method, tc.minimum, tc.latest, got, want)
			}
		}
	}
}

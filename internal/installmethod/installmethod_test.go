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
		{"go-install", MethodUnknown},
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
	// Force no build-info version so detection deterministically falls through
	// past the go-install check to unknown, regardless of how the test binary
	// was built.
	orig := readMainVersion
	t.Cleanup(func() { readMainVersion = orig })
	readMainVersion = func() string { return "" }
	// No marker file and no node_modules in the temp build-dir path either.
	if got := Detect(); got != MethodUnknown {
		t.Fatalf("Detect() = %q, want unknown", got)
	}
}

func TestDetectGoInstall(t *testing.T) {
	t.Setenv(EnvVar, "")
	orig := readMainVersion
	t.Cleanup(func() { readMainVersion = orig })
	readMainVersion = func() string { return "v0.2.2" }
	if got := Detect(); got != MethodGoInstall {
		t.Fatalf("Detect() = %q, want go-install", got)
	}

	readMainVersion = func() string { return "(devel)" }
	if got := Detect(); got == MethodGoInstall {
		t.Fatalf("Detect() = go-install for a (devel) build, want non-go-install")
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
		{MethodGoInstall, "0.2.0", []string{"(0.2.0)", "go install github.com/tollbit/cli/cmd/tollbit@latest"}},
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
		{MethodGoInstall, "0.1.0", "0.2.0", []string{"minimum: 0.1.0", "version 0.2.0", "go install github.com/tollbit/cli/cmd/tollbit@latest"}},
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

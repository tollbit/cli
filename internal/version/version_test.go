package version

import "testing"

func TestPrereleaseBuildReportsFullVersion(t *testing.T) {
	t.Cleanup(func() { prerelease = "" })
	prerelease = "preview.3"

	want := Version + "-preview.3"
	if got := String(); got != want {
		t.Fatalf("String()=%q want %q", got, want)
	}
	if got := ClientHeader(); got != Product+"/"+want {
		t.Fatalf("ClientHeader()=%q want %q", got, Product+"/"+want)
	}
	if got := HTTPUserAgent(); got != HTTPProduct+"/v"+want {
		t.Fatalf("HTTPUserAgent()=%q want %q", got, HTTPProduct+"/v"+want)
	}
}

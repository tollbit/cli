// Package version holds the public CLI and bundled-skill release number.
// Bump this when shipping; keep skill frontmatter `version` in sync (tests enforce it).
package version

const Version = "0.3.9"

// prerelease is set at build time for preview builds only, e.g.
// -ldflags "-X github.com/tollbit/cli/internal/version.prerelease=preview.3".
// Empty in public builds. Don't rename or remove it: preview builds
// (.github/workflows/preview.yml) need it, and the linker silently ignores -X
// for a variable that doesn't exist.
var prerelease string

// String returns the version this binary reports: Version, or
// Version-prerelease for a preview build.
func String() string {
	if prerelease == "" {
		return Version
	}
	return Version + "-" + prerelease
}

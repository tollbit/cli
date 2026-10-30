//go:build !dev

package plugin

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tollbit/cli/internal/version"
)

var repoRoot = filepath.Join("..", "..", "..")

func TestPluginFilesAreUpToDate(t *testing.T) {
	files, err := Files(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range files {
		got, err := os.ReadFile(filepath.Join(repoRoot, p))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s is out of date: run `make plugin` and commit the result", p)
		}
	}

	// A skill removed from skill/ must leave the plugin too.
	entries, err := os.ReadDir(filepath.Join(repoRoot, SkillsDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if _, ok := files[path.Join(SkillsDir, entry.Name(), "SKILL.md")]; !ok {
			t.Errorf("%s/%s is not generated from skill/: remove it", SkillsDir, entry.Name())
		}
	}
}

func TestGenerateWritesSkillsAndVersions(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{ManifestPath, InstallSkillPath} {
		content := `"version": "0.0.0" raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh`
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	paths, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(paths, path.Join(SkillsDir, "tollbit-cli", "SKILL.md")) {
		t.Fatalf("expected the tollbit-cli skill among %v", paths)
	}
	cliSkill, err := os.ReadFile(filepath.Join(root, SkillsDir, "tollbit-cli", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cliSkill), "{{") {
		t.Fatal("tollbit-cli skill contains unrendered template syntax")
	}

	manifest, err := os.ReadFile(filepath.Join(root, ManifestPath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), `"version": "`+version.Version+`"`) {
		t.Fatalf("manifest version not set: %s", manifest)
	}
	install, err := os.ReadFile(filepath.Join(root, InstallSkillPath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(install), "tollbit/cli/v"+version.Version+"/scripts/install.sh") {
		t.Fatalf("installer not pinned to the release tag: %s", install)
	}
}

func TestFilesRejectsVersionedFileWithoutVersion(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{ManifestPath, InstallSkillPath} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Files(root); err == nil || !strings.Contains(err.Error(), ManifestPath) {
		t.Fatalf("expected an error naming %s, got %v", ManifestPath, err)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

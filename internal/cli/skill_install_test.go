package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillNameFromMarkdown(t *testing.T) {
	name, err := skillNameFromMarkdown(skillMarkdown)
	if err != nil {
		t.Fatalf("skillNameFromMarkdown: %v", err)
	}
	if name != "tollbit-cli" {
		t.Fatalf("name=%q want tollbit-cli", name)
	}
}

func TestResolveSkillInstallDirectory(t *testing.T) {
	const skillName = "tollbit-cli"
	tmp := t.TempDir()

	t.Run("parent_skills_dir", func(t *testing.T) {
		got := resolveSkillInstallDirectory(tmp, skillName)
		want := filepath.Join(tmp, "tollbit-cli")
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("already_skill_dir", func(t *testing.T) {
		skillDir := filepath.Join(tmp, "already", skillName)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		got := resolveSkillInstallDirectory(skillDir, skillName)
		if got != filepath.Clean(skillDir) {
			t.Fatalf("got %q want %q", got, skillDir)
		}
	})

	t.Run("skill_dir_with_trailing_separator", func(t *testing.T) {
		skillDir := filepath.Join(t.TempDir(), "trail", skillName) + string(os.PathSeparator)
		got := resolveSkillInstallDirectory(skillDir, skillName)
		want := filepath.Clean(skillDir)
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
		if filepath.Base(got) != skillName {
			t.Fatalf("base %q", filepath.Base(got))
		}
	})
}

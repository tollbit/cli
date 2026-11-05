package doctor

import (
	"strings"
	"testing"
)

func TestSkillChecks(t *testing.T) {
	checks := skillChecks([]Skill{{Name: "tollbit-cli", Version: "1.2.3", Install: "tollbit guide --skill cli --install <skills-dir>"}})
	if len(checks) != 1 {
		t.Fatalf("got %d checks, want 1", len(checks))
	}
	c := checks[0]
	if c.Group != GroupSkills || c.Name != "tollbit-cli" || c.Status != StatusInfo || c.Version != "1.2.3" {
		t.Fatalf("unexpected check %+v", c)
	}
	if !strings.Contains(c.Hint, "tollbit guide --skill cli --install <skills-dir>") {
		t.Fatalf("hint %q should carry the install command", c.Hint)
	}
}

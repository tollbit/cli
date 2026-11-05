package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/tollbit/cli/skill"
)

// Skill is an agent skill bundled with this CLI. Every agent keeps skills in
// its own place, so doctor only looks for installed copies when told where
// they are (Options.SkillsDir). Otherwise it lists what this CLI ships so the
// agent can compare against the copy it loaded.
type Skill struct {
	Name    string
	Version string
	// GuideName selects the skill in `tollbit guide --skill`.
	GuideName string
}

// installCommand is the command that installs or updates the skill in dir.
func (s Skill) installCommand(dir string) string {
	return fmt.Sprintf("tollbit guide --skill %s --install %s", s.GuideName, dir)
}

func skillChecks(skills []Skill) []Check {
	checks := make([]Check, 0, len(skills))
	for _, s := range skills {
		checks = append(checks, Check{
			Group:   GroupSkills,
			Name:    s.Name,
			Status:  StatusInfo,
			Detail:  "version " + s.Version + " bundled with this CLI",
			Hint:    "update with `" + s.installCommand("<skills-dir>") + "`",
			Version: s.Version,
		})
	}
	return checks
}

// installedSkillChecks compares the copies under dir with the bundled skills.
// These checks warn and never fail: the CLI runs without skills, an agent may
// install only some of them, and a failed doctor reads as a failed install.
// If dir itself can't be checked, the report lists the bundled skills instead.
func installedSkillChecks(dir string, skills []Skill) []Check {
	root := skillsRoot(dir, skills)
	if problem := skillsDirProblem(root); problem != nil {
		return append([]Check{*problem}, skillChecks(skills)...)
	}
	checks := make([]Check, 0, len(skills))
	for _, s := range skills {
		checks = append(checks, installedSkillCheck(root, s))
	}
	return checks
}

// skillsRoot returns the folder that holds the skill folders. `tollbit guide
// --install` also accepts one skill's own folder, so dir may be that; the
// other skills are then its siblings, where an agent would find them.
func skillsRoot(dir string, skills []Skill) string {
	cleaned := filepath.Clean(dir)
	for _, s := range skills {
		if filepath.Base(cleaned) == s.Name {
			return filepath.Dir(cleaned)
		}
	}
	return cleaned
}

func skillsDirProblem(dir string) *Check {
	problem := func(detail string) *Check {
		return &Check{
			Group:  GroupSkills,
			Name:   "skills dir",
			Status: StatusWarn,
			Detail: detail,
			Hint:   "pass the folder you gave `tollbit guide --install`, which holds one folder per skill",
			Path:   dir,
		}
	}
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return problem(dir + " does not exist")
	case err != nil:
		return problem("could not read " + dir + ": " + err.Error())
	case !info.IsDir():
		return problem(dir + " is not a directory")
	}
	if _, err := os.ReadDir(dir); err != nil {
		return problem("could not read " + dir + ": " + err.Error())
	}
	return nil
}

func installedSkillCheck(root string, s Skill) Check {
	path := filepath.Join(skill.InstallDir(root, s.Name), "SKILL.md")
	install := s.installCommand(shellArg(root))
	c := Check{Group: GroupSkills, Name: s.Name, Status: StatusWarn, Path: path, Version: s.Version}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		c.Detail = "not installed at " + path
		c.Hint = "install with `" + install + "`. Skills from the TollBit plugin live elsewhere and are not checked; update the plugin instead"
		return c
	}
	if err != nil {
		c.Detail = "could not read " + path + ": " + err.Error()
		c.Hint = "check the file's permissions, or reinstall with `" + install + "`"
		return c
	}
	installed, err := skill.Version(string(data))
	if err != nil {
		c.Detail = "no version in " + path + ": " + err.Error()
		c.Hint = "reinstall with `" + install + "`"
		return c
	}
	c.InstalledVersion = installed
	if installed != s.Version {
		c.Detail = "version " + installed + " installed at " + path + ", this CLI bundles " + s.Version
		c.Hint = "update with `" + install + "`"
		return c
	}
	c.Status = StatusPass
	c.Detail = "version " + installed + " installed at " + path
	return c
}

// shellSafePath matches a path the shell leaves alone. Paths here are
// absolute, so a ~ can't start one and expand.
var shellSafePath = regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,~-]+$`)

// shellArg quotes a path the shell would split or expand, so the hint can be
// pasted as a command. Windows has no single-quote form in cmd.exe, and its
// path separator is safe there.
func shellArg(path string) string {
	if runtime.GOOS == "windows" {
		if shellSafePath.MatchString(strings.ReplaceAll(path, `\`, "/")) {
			return path
		}
		return `"` + path + `"`
	}
	if shellSafePath.MatchString(path) {
		return path
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

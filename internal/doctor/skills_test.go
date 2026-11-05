package doctor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSkillChecks(t *testing.T) {
	checks := skillChecks([]Skill{{Name: "tollbit-cli", Version: "1.2.3", GuideName: "cli"}})
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

const bundledVersion = "1.2.3"

func testSkills() []Skill {
	return []Skill{
		{Name: "tollbit-cli", Version: bundledVersion, GuideName: "cli"},
		{Name: "tollbit-analytics", Version: bundledVersion, GuideName: "analytics"},
	}
}

func writeSkill(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func skillBody(version string) string {
	return "---\nname: any\nversion: " + version + "\n---\n\nversion: not-this-one\n"
}

func TestInstalledSkillChecks(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "tollbit-cli", skillBody(bundledVersion))

	cases := []struct {
		name          string
		analytics     string // SKILL.md of tollbit-analytics; empty leaves it out
		wantStatus    Status
		wantInstalled string
		wantDetail    string
		wantHint      string
	}{
		{
			name:       "missing",
			wantStatus: StatusWarn,
			wantDetail: "not installed at ",
			wantHint:   "install with `tollbit guide --skill analytics --install " + dir + "`",
		},
		{
			name:          "stale",
			analytics:     skillBody("1.0.0"),
			wantStatus:    StatusWarn,
			wantInstalled: "1.0.0",
			wantDetail:    "version 1.0.0 installed at ",
			wantHint:      "update with `tollbit guide --skill analytics --install " + dir + "`",
		},
		{
			name:       "no version",
			analytics:  "---\nname: tollbit-analytics\n---\n",
			wantStatus: StatusWarn,
			wantDetail: "no version in ",
			wantHint:   "reinstall with `tollbit guide --skill analytics --install " + dir + "`",
		},
		{
			name:          "matching",
			analytics:     skillBody(bundledVersion),
			wantStatus:    StatusPass,
			wantInstalled: bundledVersion,
			wantDetail:    "version 1.2.3 installed at ",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.RemoveAll(filepath.Join(dir, "tollbit-analytics"))
			if tc.analytics != "" {
				writeSkill(t, dir, "tollbit-analytics", tc.analytics)
			}

			checks := installedSkillChecks(dir, testSkills())
			if len(checks) != 2 {
				t.Fatalf("got %d checks, want 2: %+v", len(checks), checks)
			}
			if checks[0].Status != StatusPass {
				t.Fatalf("tollbit-cli should pass: %+v", checks[0])
			}
			c := checks[1]
			want := filepath.Join(dir, "tollbit-analytics", "SKILL.md")
			if c.Group != GroupSkills || c.Name != "tollbit-analytics" || c.Status != tc.wantStatus || c.Path != want ||
				c.Version != bundledVersion || c.InstalledVersion != tc.wantInstalled {
				t.Fatalf("unexpected check %+v", c)
			}
			if !strings.Contains(c.Detail, tc.wantDetail) {
				t.Fatalf("detail %q should contain %q", c.Detail, tc.wantDetail)
			}
			if (tc.wantHint == "") != (c.Hint == "") || !strings.Contains(c.Hint, tc.wantHint) {
				t.Fatalf("hint %q should contain %q", c.Hint, tc.wantHint)
			}
		})
	}
}

func TestInstalledSkillChecksFromOneSkillsFolder(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "tollbit-cli", skillBody(bundledVersion))
	writeSkill(t, dir, "tollbit-analytics", skillBody(bundledVersion))

	checks := installedSkillChecks(filepath.Join(dir, "tollbit-cli")+string(os.PathSeparator), testSkills())

	for _, c := range checks {
		if c.Status != StatusPass {
			t.Fatalf("expected both skills found beside the folder passed, got %+v", c)
		}
	}
}

func TestInstalledSkillChecksQuotesPathsInHints(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows paths can't hold these characters")
	}
	cases := []struct{ name, folder, want string }{
		{name: "space", folder: "my skills", want: "'my skills'"},
		{name: "dollar", folder: "$HOME", want: "'$HOME'"},
		{name: "backtick", folder: "a`b", want: "'a`b'"},
		{name: "double quote", folder: `a"b`, want: `'a"b'`},
		{name: "single quote", folder: "it's", want: `'it'\''s'`},
		{name: "backslash", folder: `a\b`, want: `'a\b'`},
		{name: "bang", folder: "a!b", want: "'a!b'"},
		{name: "hash", folder: "a#b", want: "'a#b'"},
		{name: "brace", folder: "{a,b}", want: "'{a,b}'"},
		{name: "newline", folder: "a\nb", want: "'a\nb'"},
		{name: "safe", folder: "a-b_c.d", want: "a-b_c.d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parent := t.TempDir()
			if err := os.Mkdir(filepath.Join(parent, tc.folder), 0o755); err != nil {
				t.Fatal(err)
			}

			checks := installedSkillChecks(filepath.Join(parent, tc.folder), testSkills())

			want := "--install " + tc.want
			got := strings.ReplaceAll(checks[0].Hint, parent+string(os.PathSeparator), "")
			if !strings.Contains(got, want+"`") {
				t.Fatalf("hint %q should contain %q", checks[0].Hint, want)
			}
		})
	}
}

func TestInstalledSkillChecksBadDirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(t.TempDir(), "unreadable")
	if err := os.Mkdir(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(unreadable, 0o755) })

	cases := []struct {
		name, dir, wantDetail string
		skip                  bool
	}{
		{name: "missing", dir: filepath.Join(t.TempDir(), "nope"), wantDetail: "does not exist"},
		{name: "file", dir: file, wantDetail: "is not a directory"},
		{name: "unreadable", dir: unreadable, wantDetail: "could not read", skip: runtime.GOOS == "windows" || os.Geteuid() == 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.Skip("this user can read any directory")
			}
			checks := installedSkillChecks(tc.dir, testSkills())
			if len(checks) != 3 {
				t.Fatalf("got %d checks, want the directory and both skills: %+v", len(checks), checks)
			}
			c := checks[0]
			if c.Name != "skills dir" || c.Status != StatusWarn || c.Path != tc.dir || !strings.Contains(c.Detail, tc.wantDetail) {
				t.Fatalf("unexpected check %+v", c)
			}
			if c.Hint == "" {
				t.Fatal("expected a hint")
			}
			if checks[1].Status != StatusInfo || checks[2].Status != StatusInfo {
				t.Fatalf("expected the bundled skills listed, got %+v", checks[1:])
			}
		})
	}
}

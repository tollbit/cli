// Package plugin generates the Claude plugin's files from the CLI: every
// skill the CLI embeds, and the CLI version in the plugin's hand-written
// files. cmd/gen-plugin (make plugin) runs it; the tollbit binary doesn't
// import it.
package plugin

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"

	tollbitcli "github.com/tollbit/cli"
	"github.com/tollbit/cli/internal/app"
	"github.com/tollbit/cli/internal/cli"
	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/version"
	"github.com/tollbit/cli/skill"
)

// The Claude plugin's files, relative to the repo root. Skills under SkillsDir
// are generated from skill/, except InstallSkillPath, which is written by hand
// for the plugin only.
const (
	SkillsDir        = "plugins/tollbit/skills"
	ManifestPath     = "plugins/tollbit/.claude-plugin/plugin.json"
	InstallSkillPath = "plugins/tollbit/skills/tollbit-install-claude/SKILL.md"
)

// versioned are hand-written plugin files with one generated part: the CLI
// version.
var versioned = []struct {
	path    string
	pattern *regexp.Regexp
	format  string
}{
	{ManifestPath, regexp.MustCompile(`"version":\s*"[^"]*"`), `"version": "%s"`},
	// Pin the installer to this release's tag, not the moving main branch.
	{InstallSkillPath, regexp.MustCompile(`raw\.githubusercontent\.com/tollbit/cli/[^/]+/scripts/install\.sh`), "raw.githubusercontent.com/tollbit/cli/v%s/scripts/install.sh"},
}

// Files returns the content every generated plugin file should have, keyed by
// its path relative to the repo root. It reads the hand-written files from
// root. Dev builds can change the auth configuration the CLI skill is rendered
// with, so it refuses to run in one.
func Files(root string) (map[string][]byte, error) {
	if configuration.IsDev {
		return nil, errors.New("generate the plugin from a release build (without -tags dev)")
	}
	files, err := skills()
	if err != nil {
		return nil, err
	}
	for _, v := range versioned {
		content, err := os.ReadFile(filepath.Join(root, v.path))
		if err != nil {
			return nil, err
		}
		if !v.pattern.Match(content) {
			return nil, fmt.Errorf("%s has nothing matching %s", v.path, v.pattern)
		}
		files[v.path] = v.pattern.ReplaceAll(content, []byte(fmt.Sprintf(v.format, version.Version)))
	}
	return files, nil
}

// Generate writes Files(root) under root and returns the paths it wrote, in
// order.
func Generate(root string) ([]string, error) {
	files, err := Files(root)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		target := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, files[p], 0o644); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// skills renders every skill the CLI embeds, keyed by its path under
// SkillsDir. Each is exactly what `tollbit guide --skill <name>` prints from a
// release build.
func skills() (map[string][]byte, error) {
	names, err := skill.Names()
	if err != nil {
		return nil, err
	}
	rendered := make(map[string][]byte, len(names))
	for _, name := range names {
		markdown, err := releaseGuide(name)
		if err != nil {
			return nil, err
		}
		rendered[path.Join(SkillsDir, name, "SKILL.md")] = []byte(markdown)
	}
	return rendered, nil
}

// releaseGuide runs `tollbit guide --skill <name>` with the embedded release
// configuration.
func releaseGuide(name string) (string, error) {
	config, err := configuration.AssembleConfiguration(tollbitcli.DefaultConfig)
	if err != nil {
		return "", err
	}
	// Keep the user's runtime state and credentials out of the render.
	dir, err := os.MkdirTemp("", "tollbit-plugin-skill-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	config.Runtime.StateDir = dir
	config.Credentials.StorageDir = dir

	var stdout, stderr bytes.Buffer
	cmd := cli.NewCommandTree(app.Factory{Config: config})
	cmd.SetArgs([]string{"guide", "--skill", name})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		return "", fmt.Errorf("tollbit guide --skill %s: %w: %s", name, err, stderr.String())
	}
	return stdout.String(), nil
}

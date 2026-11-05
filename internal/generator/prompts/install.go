package prompts

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tollbit/cli/internal/version"
)

// Paths relative to the repo root.
const (
	PlatformsPath = "platforms/supported-platforms.yaml"
	TemplatePath  = "prompts/install-prompt.template.md"
	PromptPath    = "prompts/install-prompt.md"
)

// The template's hints table placeholder, and the heading where the published
// prompt ends.
const (
	hintsPlaceholder = "{{hints_table}}"
	todoHeading      = "\n## TODO\n"
)

type (
	platformList struct {
		Platforms []platform `yaml:"platforms"`
	}

	platform struct {
		Name      string `yaml:"name"`
		Detection string `yaml:"detection"`
		Install   string `yaml:"install"`
	}
)

// installSections maps a platforms file install value to its section title in the
// template ("### Install: <title>").
var installSections = map[string]string{
	"script":        "script",
	"claude-plugin": "Claude plugin",
	"mcp":           "MCP (coming soon)",
	"not-supported": "not supported",
}

// installFiles returns the generated install prompt, keyed by its path
// relative to the repo root.
func installFiles(root string) (map[string][]byte, error) {
	template, err := os.ReadFile(filepath.Join(root, TemplatePath))
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(root, PlatformsPath))
	if err != nil {
		return nil, err
	}
	var m platformList
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", PlatformsPath, err)
	}
	return render(template, m, version.Version)
}

// render builds the published prompt: the template up to its TODO section, with the hints table filled in from the
// platforms file and {{version}} set to the release.
func render(template []byte, m platformList, release string) (map[string][]byte, error) {
	if err := check(template, m); err != nil {
		return nil, err
	}
	body, _, found := bytes.Cut(template, []byte(todoHeading))
	if !found {
		return nil, fmt.Errorf("%s has no %q heading", TemplatePath, strings.TrimSpace(todoHeading))
	}
	prompt := strings.TrimRight(string(body), "\n") + "\n"
	prompt = strings.Replace(prompt, hintsPlaceholder, hintsTable(m), 1)
	filled, err := fill(TemplatePath, prompt, release)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{PromptPath: filled}, nil
}

// check reports platforms the template can't render.
func check(template []byte, m platformList) error {
	if !bytes.Contains(template, []byte(hintsPlaceholder)) {
		return fmt.Errorf("%s has no %s", TemplatePath, hintsPlaceholder)
	}
	for _, title := range installSections {
		if !bytes.Contains(template, []byte("\n### Install: "+title+"\n")) {
			return fmt.Errorf("%s has no \"### Install: %s\" section", TemplatePath, title)
		}
	}
	for _, p := range m.Platforms {
		if p.Install != "" {
			if _, ok := installSections[p.Install]; !ok {
				return fmt.Errorf("%s: %s has install %q, which names no install section", PlatformsPath, p.Name, p.Install)
			}
		}
		if p.Detection != "" && p.Install == "" {
			return fmt.Errorf("%s: %s has a detection line but no install", PlatformsPath, p.Name)
		}
	}
	return nil
}

// hintsTable renders the platforms that have a detection line, in file
// order. Platforms with the same detection and install share a row.
func hintsTable(m platformList) string {
	type row struct {
		names              []string
		detection, install string
	}
	var rows []*row
	byKey := map[string]*row{}
	for _, p := range m.Platforms {
		if p.Detection == "" {
			continue
		}
		key := p.Detection + "\x00" + p.Install
		if r, ok := byKey[key]; ok {
			r.names = append(r.names, p.Name)
			continue
		}
		r := &row{names: []string{p.Name}, detection: p.Detection, install: p.Install}
		byKey[key] = r
		rows = append(rows, r)
	}

	var b strings.Builder
	b.WriteString("   | Platform | What we saw | Install |\n   |---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "   | %s | %s | Install: %s |\n", strings.Join(r.names, " or "), r.detection, installSections[r.install])
	}
	b.WriteString("   | Any other agent with no shell | only your instructions | Install: " + installSections["not-supported"] + " |")
	return b.String()
}

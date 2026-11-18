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

// The template's placeholders for the hints table and the troubleshooting
// steps.
const (
	hintsPlaceholder        = "{{hints_table}}"
	troubleshootPlaceholder = "{{troubleshooting}}"
)

type (
	platformList struct {
		Platforms []platform `yaml:"platforms"`
	}

	platform struct {
		Name      string `yaml:"name"`
		Detection string `yaml:"detection"`
		Install   string `yaml:"install"`
		Fallback  string `yaml:"fallback"`
	}
)

// installSections maps a platforms file install value to its section title in the
// template ("### Install: <title>").
var installSections = map[string]string{
	"script":        "script",
	"codex":         "Codex",
	"mcp":           "MCP",
	"mcp-instead":   "MCP instead",
	"mcp-admin":     "MCP through an admin",
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
	troubleshoot, err := os.ReadFile(filepath.Join(root, TroubleshootTemplatePath))
	if err != nil {
		return nil, err
	}
	return render(template, troubleshoot, m, version.Version)
}

// render builds the published prompt: the template with the hints table
// filled in from the platforms file, the troubleshooting template embedded,
// and {{version}} set to the release.
func render(template, troubleshoot []byte, m platformList, release string) (map[string][]byte, error) {
	if err := check(template, m); err != nil {
		return nil, err
	}
	steps, err := embedTroubleshooting(troubleshoot)
	if err != nil {
		return nil, err
	}
	prompt := strings.TrimRight(string(template), "\n") + "\n"
	prompt = strings.Replace(prompt, hintsPlaceholder, hintsTable(m), 1)
	prompt = strings.Replace(prompt, troubleshootPlaceholder, steps, 1)
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
	if !bytes.Contains(template, []byte("\n"+troubleshootPlaceholder+"\n")) {
		return fmt.Errorf("%s has no %s on a line of its own", TemplatePath, troubleshootPlaceholder)
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
		if p.Fallback != "" {
			if _, ok := installSections[p.Fallback]; !ok {
				return fmt.Errorf("%s: %s has fallback %q, which names no install section", PlatformsPath, p.Name, p.Fallback)
			}
			if p.Install == "" {
				return fmt.Errorf("%s: %s has a fallback but no install", PlatformsPath, p.Name)
			}
		}
		if p.Detection != "" && p.Install == "" {
			return fmt.Errorf("%s: %s has a detection line but no install", PlatformsPath, p.Name)
		}
	}
	return nil
}

func embedTroubleshooting(template []byte) (string, error) {
	title, body, _ := strings.Cut(strings.TrimSpace(string(template)), "\n")
	if !strings.HasPrefix(title, "# ") {
		return "", fmt.Errorf("%s doesn't start with a \"# \" title", TroubleshootTemplatePath)
	}
	lines := strings.Split(strings.TrimSpace(body), "\n")
	inCode := false
	for i, line := range lines {
		switch {
		case strings.HasPrefix(strings.TrimSpace(line), "```"):
			inCode = !inCode
		case !inCode && strings.HasPrefix(line, "#"):
			lines[i] = "#" + line
		}
	}
	if inCode {
		return "", fmt.Errorf("%s has a code block that never closes", TroubleshootTemplatePath)
	}
	return strings.Join(lines, "\n"), nil
}

// hintsTable renders the platforms that have a detection line, in file
// order. Platforms with the same detection, install and fallback share a row.
func hintsTable(m platformList) string {
	type row struct {
		names                        []string
		detection, install, fallback string
	}
	var rows []*row
	byKey := map[string]*row{}
	for _, p := range m.Platforms {
		if p.Detection == "" {
			continue
		}
		key := p.Detection + "\x00" + p.Install + "\x00" + p.Fallback
		if r, ok := byKey[key]; ok {
			r.names = append(r.names, p.Name)
			continue
		}
		r := &row{names: []string{p.Name}, detection: p.Detection, install: p.Install, fallback: p.Fallback}
		byKey[key] = r
		rows = append(rows, r)
	}

	var b strings.Builder
	b.WriteString("   | Platform | What to look for | Install |\n   |---|---|---|\n")
	for _, r := range rows {
		install := "Install: " + installSections[r.install]
		if r.fallback != "" {
			install += ", then Install: " + installSections[r.fallback]
		}
		fmt.Fprintf(&b, "   | %s | %s | %s |\n", strings.Join(r.names, " or "), r.detection, install)
	}
	b.WriteString("   | Any other agent with no shell | only your instructions | Install: " + installSections["mcp"] + " if the user can add a custom MCP server there, otherwise Install: " + installSections["not-supported"] + " |")
	return b.String()
}

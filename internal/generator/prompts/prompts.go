// Package prompts generates the prompts in prompts/ that agents are given,
// each from its template (prompts/*.template.md): the install prompt, with its
// hints table from the platforms file, and the install troubleshooting prompt.
// Templates name the release as {{version}}. cmd/gen-prompts (make prompts)
// runs it.
package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tollbit/cli/internal/version"
)

// The install troubleshooting prompt and its template, relative to the repo
// root. The plugin's troubleshooting skill is generated from the same
// rendering (Troubleshoot).
const (
	TroubleshootTemplatePath = "prompts/install-troubleshooting.template.md"
	TroubleshootPromptPath   = "prompts/install-troubleshooting.md"
)

// versionPlaceholder marks where a template names the release.
const versionPlaceholder = "{{version}}"

// fill sets the release in a rendered template, and refuses one that still
// has a placeholder.
func fill(path, rendered, release string) ([]byte, error) {
	rendered = strings.ReplaceAll(rendered, versionPlaceholder, release)
	if strings.Contains(rendered, "{{") {
		return nil, fmt.Errorf("%s has an unknown placeholder", path)
	}
	return []byte(rendered), nil
}

// Troubleshoot renders the install troubleshooting prompt for this release.
func Troubleshoot(root string) ([]byte, error) {
	template, err := os.ReadFile(filepath.Join(root, TroubleshootTemplatePath))
	if err != nil {
		return nil, err
	}
	return fill(TroubleshootTemplatePath, string(template), version.Version)
}

// Files returns the content every generated prompt should have, keyed by its
// path relative to the repo root.
func Files(root string) (map[string][]byte, error) {
	files, err := installFiles(root)
	if err != nil {
		return nil, err
	}
	troubleshoot, err := Troubleshoot(root)
	if err != nil {
		return nil, err
	}
	files[TroubleshootPromptPath] = troubleshoot
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

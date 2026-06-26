package cli

import (
	"fmt"
	"path/filepath"
	"strings"
)

// skillNameFromMarkdown returns the `name:` field from YAML frontmatter (delimited by the
// first two standalone --- lines). Body markdown may contain extra --- horizontal rules;
// we must not parse those as frontmatter delimiters.
func skillNameFromMarkdown(md string) (string, error) {
	md = strings.TrimPrefix(md, "\ufeff")
	normalized := strings.ReplaceAll(md, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", fmt.Errorf("skill markdown: frontmatter must start with ---")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			fm := strings.Join(lines[1:i], "\n")
			return nameFromYAMLFrontmatter(fm)
		}
	}
	return "", fmt.Errorf("skill markdown: missing closing --- for frontmatter")
}

func nameFromYAMLFrontmatter(fm string) (string, error) {
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			val = strings.Trim(val, `"'`)
			if val == "" {
				return "", fmt.Errorf("skill markdown: empty name in frontmatter")
			}
			return val, nil
		}
	}
	return "", fmt.Errorf("skill markdown: name field not found in frontmatter")
}

// resolveSkillInstallDirectory turns a user-supplied path into the directory that should
// contain SKILL.md. If arg already names the skill folder (last segment equals skillName),
// it is returned as-is; otherwise skillName is appended (parent skills directory case).
func resolveSkillInstallDirectory(arg string, skillName string) string {
	cleaned := filepath.Clean(arg)
	if filepath.Base(cleaned) == skillName {
		return cleaned
	}
	return filepath.Join(cleaned, skillName)
}

package skill

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Name returns the `name:` field of a skill's YAML frontmatter.
func Name(md string) (string, error) {
	return frontmatterField(md, "name")
}

// Version returns the `version:` field of a skill's YAML frontmatter.
func Version(md string) (string, error) {
	return frontmatterField(md, "version")
}

// frontmatterField reads a field from the YAML frontmatter, delimited by the first two
// standalone --- lines. Body markdown may contain extra --- horizontal rules; we must not
// parse those as frontmatter delimiters.
func frontmatterField(md, field string) (string, error) {
	md = strings.TrimPrefix(md, "\ufeff")
	normalized := strings.ReplaceAll(md, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", fmt.Errorf("skill markdown: frontmatter must start with ---")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return fieldFromYAMLFrontmatter(lines[1:i], field)
		}
	}
	return "", fmt.Errorf("skill markdown: missing closing --- for frontmatter")
}

func fieldFromYAMLFrontmatter(lines []string, field string) (string, error) {
	prefix := field + ":"
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			val = strings.Trim(val, `"'`)
			if val == "" {
				return "", fmt.Errorf("skill markdown: empty %s in frontmatter", field)
			}
			return val, nil
		}
	}
	return "", fmt.Errorf("skill markdown: %s field not found in frontmatter", field)
}

// InstallDir turns a user-supplied path into the directory that should contain SKILL.md.
// If arg already names the skill folder (last segment equals skillName), it is returned
// as-is; otherwise skillName is appended (parent skills directory case).
func InstallDir(arg string, skillName string) string {
	cleaned := filepath.Clean(arg)
	if filepath.Base(cleaned) == skillName {
		return cleaned
	}
	return filepath.Join(cleaned, skillName)
}

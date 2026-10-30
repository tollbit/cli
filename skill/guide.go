package skill

import (
	"embed"
	"io/fs"
	"path"
)

// TollbitCLISkill is the canonical agent guide markdown.
//
//go:embed tollbit-cli/SKILL.md
var TollbitCLISkill string

// TollbitAnalyticsSkill is the canonical agent guide markdown for analytics.
//
//go:embed tollbit-analytics/SKILL.md
var TollbitAnalyticsSkill string

//go:embed */SKILL.md
var skills embed.FS

// Names returns the directory name of every skill this package embeds, which
// is also its name for `tollbit guide --skill`. The Claude plugin ships each
// one (internal/generator/plugin).
func Names() ([]string, error) {
	paths, err := fs.Glob(skills, "*/SKILL.md")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(paths))
	for _, p := range paths {
		names = append(names, path.Dir(p))
	}
	return names, nil
}

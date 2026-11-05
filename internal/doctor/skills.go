package doctor

// Skill is an agent skill bundled with this CLI. Doctor does not look for
// installed copies, since every agent keeps skills in its own place; it lists
// what this CLI ships so the agent can compare against the copy it loaded.
type Skill struct {
	Name    string
	Version string
	// Install is the command that installs or updates the skill.
	Install string
}

func skillChecks(skills []Skill) []Check {
	checks := make([]Check, 0, len(skills))
	for _, s := range skills {
		checks = append(checks, Check{
			Group:   GroupSkills,
			Name:    s.Name,
			Status:  StatusInfo,
			Detail:  "version " + s.Version + " bundled with this CLI",
			Hint:    "update with `" + s.Install + "`",
			Version: s.Version,
		})
	}
	return checks
}

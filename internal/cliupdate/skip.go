package cliupdate

import "github.com/spf13/cobra"

// skippedCommands never run the update check: they print local information an
// outdated CLI can still give correctly, and shell completion must stay
// silent and fast.
var skippedCommands = map[string]bool{
	"version":                       true,
	"help":                          true,
	"completion":                    true,
	cobra.ShellCompRequestCmd:       true,
	cobra.ShellCompNoDescRequestCmd: true,
}

// Skip reports whether cmd runs without the update check: the root command
// alone (help or --version), and the commands in skippedCommands and their
// subcommands.
func Skip(cmd *cobra.Command) bool {
	if !cmd.HasParent() {
		return true
	}
	for c := cmd; c.HasParent(); c = c.Parent() {
		if skippedCommands[c.Name()] {
			return true
		}
	}
	return false
}

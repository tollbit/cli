package cli

import (
	"github.com/spf13/cobra"
	"github.com/tollbit/tollbit-cli/internal/app"
)

func NewCommandTree(factory app.Factory) *cobra.Command {
	rootCmd := NewRootCommand(factory)
	rootCmd.AddCommand(
		NewAgentCommand(factory),
		NewContentCommand(factory),
		NewIdentityCommand(factory),
		NewSearchCommand(factory),
		NewGuideCommand(),
		NewVersionCommand(),
	)
	return rootCmd
}

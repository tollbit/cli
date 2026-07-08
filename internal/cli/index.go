package cli

import (
	"github.com/spf13/cobra"
	"github.com/tollbit/tollbit-cli/internal/app"
)

func NewCommandTree(factory app.Factory) *cobra.Command {
	rootCmd := NewRootCommand(factory)
	rootCmd.AddCommand(
		NewAuthCommand(factory),
		NewContentCommand(factory),
		NewSearchCommand(factory),
		NewGuideCommand(),
		NewVersionCommand(),
	)
	return rootCmd
}

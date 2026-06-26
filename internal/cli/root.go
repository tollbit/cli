package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tollbit/tollbit-cli/internal/app"
	"github.com/tollbit/tollbit-cli/internal/cli/globalflags"
	"github.com/tollbit/tollbit-cli/internal/version"
)

func NewRootCommand(factory app.Factory) *cobra.Command {
	var showVersion bool
	cmd := &cobra.Command{
		Use:           "tollbit",
		Short:         "Tollbit CLI",
		Long:          fmt.Sprintf("Tollbit CLI\nversion: %s\n\nAgent? Run `tollbit guide` for orientation, then `tollbit guide --install <SKILLS_DIR>` to register the bundled skill.", version.Version),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), version.Version)
				return nil
			}
			if err := cmd.Help(); err != nil {
				return RuntimeError(err)
			}
			return UsageError("missing command")
		},
	}
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return UsageError("%s", err.Error())
	})
	globalflags.Add(cmd, factory.Config)
	cmd.Flags().BoolVarP(&showVersion, "version", "V", false, "print version")
	return cmd
}

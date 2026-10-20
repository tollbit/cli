package cli

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/tollbit/cli/internal/app"
	"github.com/tollbit/cli/internal/cli/globalflags"
	"github.com/tollbit/cli/internal/cliupdate"
	"github.com/tollbit/cli/internal/version"
)

func NewRootCommand(factory app.Factory) *cobra.Command {
	var showVersion bool
	cmd := &cobra.Command{
		Use:           "tollbit",
		Short:         "TollBit CLI",
		Long:          fmt.Sprintf("TollBit CLI\nversion: %s\n\nAgent? Run `tollbit guide` for orientation, then `tollbit guide --install <SKILLS_DIR>` to register the bundled skill.", version.String()),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return checkForUpdate(factory, cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), version.String())
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

// checkForUpdate runs the version check before every command that
// cliupdate.Skip does not exempt. Problems building the check are logged at
// debug level and never block the command; only the version policy's verdict
// can.
func checkForUpdate(factory app.Factory, cmd *cobra.Command) error {
	// check_enabled is not configurable at run time, so the factory's config
	// is final.
	if factory.Process == nil || !factory.Config.Update.CheckEnabled || cliupdate.Skip(cmd) {
		return nil
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	checker, err := factory.Process.UpdateChecker()
	if err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("update check skipped")
		return nil
	}
	return checker.Run(ctx)
}

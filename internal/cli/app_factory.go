package cli

import (
	"github.com/spf13/cobra"
	"github.com/tollbit/cli/internal/app"
	"github.com/tollbit/cli/internal/cli/globalflags"
)

func appForCommand(factory app.Factory, cmd *cobra.Command) (*app.App, error) {
	overrides, err := globalflags.OverridesFromCommand(cmd)
	if err != nil {
		return nil, err
	}
	return factory.New(overrides)
}

package cli

import (
	"github.com/spf13/cobra"
	"github.com/tollbit/cli/internal/app"
)

func NewContentCommand(factory app.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "content",
		Short: "Price and Fetch licensed publisher content",
		Long:  "Price and fetch licensed publisher content on the TollBit network. Every fetch charges money.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return UsageError("content requires a subcommand: fetch or pricing")
			}
			return UsageError("unknown content command %q", args[0])
		},
	}
	cmd.AddCommand(newFetchCommand(factory))
	cmd.AddCommand(newPricingCommand(factory))
	return cmd
}

package cli

import (
	"github.com/spf13/cobra"
	"github.com/tollbit/tollbit-cli/internal/app"
)

func NewContentCommand(factory app.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "content",
		Short: "Work with publisher content",
		Long:  "Discover, price, and retrieve publisher content on the TollBit network.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return UsageError("content requires pricing")
			}
			return UsageError("unknown content command %q", args[0])
		},
	}
	cmd.AddCommand(newPricingCommand(factory))
	return cmd
}

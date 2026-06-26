package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func flagChangedStr(cmd *cobra.Command, name string) *string {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	value, err := cmd.Flags().GetString(name)
	if err != nil {
		return nil
	}
	value = strings.TrimSpace(value)
	return &value
}

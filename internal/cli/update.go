package cli

import (
	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/update"
)

var updateSelf bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update dargstack to the latest version",
	Long:  "Downloads and installs the latest release of dargstack. Requires --self flag.",
	RunE:  runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateSelf, "self", false, "update dargstack itself")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	if !updateSelf {
		return cmd.Help()
	}
	return update.SelfUpdate()
}

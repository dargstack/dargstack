package cli

import (
	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/internal/update"
)

var updateSelf bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update components",
	Long:  "Update dargstack and related components.",
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

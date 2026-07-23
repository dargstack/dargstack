package cli

import (
	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/update"
)

var updateSelf bool
var updateNoSkill bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update dargstack to the latest version",
	Long:  "Downloads and installs the latest release of dargstack. Requires --self flag.",
	RunE:  runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateSelf, "self", false, "update dargstack itself")
	updateCmd.Flags().BoolVar(&updateNoSkill, "no-skill", false, "skip updating the agent skill")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	if !updateSelf {
		return cmd.Help()
	}
	if err := update.SelfUpdate(); err != nil {
		return err
	}
	if !updateNoSkill {
		autoUpdateSkill()
	}
	return nil
}

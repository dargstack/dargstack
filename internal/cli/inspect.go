package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/logger"
)

var inspectCmd = &cobra.Command{
	Use:    "inspect [timestamp]",
	Short:  "View deployment audit log",
	Long:   "View the final composed YAML that was deployed.\n\nWithout arguments, shows the latest deployment.",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, logger.StyleWarn.Render("Warning: 'inspect' is deprecated, use 'audit' instead."))
		return runAudit(cmd, args)
	},
}

func init() {
	inspectCmd.Flags().BoolVar(&auditDiff, "difference", false, "show diff between current and last deployed")
	inspectCmd.Flags().StringVar(&auditEnv, "environment", "development", "environment to audit (development or production)")
	inspectCmd.Flags().BoolVar(&auditList, "list", false, "list all past deployments")
}

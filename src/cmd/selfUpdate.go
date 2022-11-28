package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var selfUpdateCmd = &cobra.Command{
	Use:   "selfUpdate",
	Short: "Updates dargstack.",
	Long:  "Updates dargstack.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("selfUpdate called")
	},
}

func init() {
	rootCmd.AddCommand(selfUpdateCmd)
}

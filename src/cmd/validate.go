package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/azolus/dargstack/core"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Checks for an up to date README.",
	Long:  "Checks for an up to date README.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Validating", core.Hi1("README.md"), "...")

		dirName, _, _ := core.GetProjectNameAndOwner()
		pwd, e := os.Getwd()
		if e != nil {
			log.Fatal(e)
		}

		core.DockerSudo("default", "run", "--rm", "-v",
			fmt.Sprintf("%s:/mnt/%s", pwd, dirName),
			"dargmuesli/dargstack_rgen", "--path",
			fmt.Sprintf("/mnt/%s", dirName),
			"-v",
		)
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

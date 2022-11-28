package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/azolus/dargstack/core"
	"github.com/spf13/cobra"
)

var rgenCmd = &cobra.Command{
	Use:   "rgen",
	Short: "Generate the README.",
	Long:  "Generate the README.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Creating", core.Hi1("README.md"), "...")

		dirName, _, _ := core.GetProjectNameAndOwner()
		pwd, e := os.Getwd()
		if e != nil {
			log.Fatal(e)
		}

		readMe := core.DockerSudo(
			"default",
			"run",
			"--rm",
			"-v",
			fmt.Sprintf("%s:/mnt/%s", pwd, dirName),
			"dargmuesli/dargstack_rgen",
			"--path",
			fmt.Sprintf("/mnt/%s", dirName),
		)

		file, e1 := os.OpenFile(pwd+"/README.md", os.O_RDWR|os.O_CREATE, 0o755)
		if e1 != nil {
			log.Fatal(e1)
		}

		_, e2 := file.Write(readMe)
		if e2 != nil {
			log.Fatal(e2)
		}

		if e2 := file.Close(); e2 != nil {
			log.Fatal(e2)
		}
	},
}

func init() {
	rootCmd.AddCommand(rgenCmd)
}

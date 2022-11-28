package cmd

import (
	"fmt"
	"time"

	"github.com/azolus/dargstack/core"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Removes the stack.",
	Long:  "Removes the stack.",
	Run: func(cmd *cobra.Command, args []string) {
		_, project, _ := core.GetProjectNameAndOwner()
		remove(project)
	},
}

func init() {
	rootCmd.AddCommand(rmCmd)
}

func remove(project string) {
	fmt.Println("Removing stack", core.Hi1(project), "...")
	core.DockerSudo("default", "stack", "rm", project)

	fmt.Println("Waiting for stack to vanish ...")
	for core.IsStackRunning(project) {
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println(core.Succ("Done."))
}

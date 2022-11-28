package cmd

import (
	"log"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dargstack",
	Short: "A template for Docker stack project layouts",
	Long: `A template for Docker stack project layouts.
Bootstrap it from <github.com/dargmuesli/dargstack_template>!

dargstack solves the problem of separated development and production
environments in the otherwise well-defined, containerized software development
process. It focuses on the development configuration, derives the production
configuration from it and makes deployments a breeze!`,
}

func Execute() {
	cc.Init(&cc.Config{
		RootCmd:  rootCmd,
		Headings: cc.Yellow + cc.Bold,
		Commands: cc.HiBlue + cc.Bold,
		Example:  cc.Red,
		ExecName: cc.Bold,
		Flags:    cc.Bold,
	})
	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolP("offline", "o", false,
		`Do not try to update the checkout.`)
}

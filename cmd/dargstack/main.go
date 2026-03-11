package main

import (
	"os"

	"github.com/dargstack/dargstack/v4/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

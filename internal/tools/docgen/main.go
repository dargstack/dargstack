package main

import (
	"log"
	"os"

	"github.com/spf13/cobra/doc"

	"github.com/dargstack/dargstack/internal/cli"
)

func main() {
	out := "./docs"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}

	if err := os.MkdirAll(out, 0o755); err != nil {
		log.Fatal(err)
	}

	root := cli.Root()
	root.DisableAutoGenTag = true

	if err := doc.GenMarkdownTree(root, out); err != nil {
		log.Fatal(err)
	}
}

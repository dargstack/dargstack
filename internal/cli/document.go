package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/internal/config"
	"github.com/dargstack/dargstack/internal/resource"
)

var docsCmd = &cobra.Command{
	Use:     "document",
	Aliases: []string{"docs"},
	Short:   "Generate the stack documentation",
	Long: `Generate stack documentation.

Creates a README.md in the artifacts directory listing all services
found in compose files, along with YAML comments describing each.
Includes a link to the stack domain and source code repository.`,
	RunE: runDocs,
}

func runDocs(_ *cobra.Command, _ []string) error {
	outputDir := config.ArtifactsDir(stackDir)
	docsDir := filepath.Join(outputDir, "docs")
	content, err := resource.GenerateDocumentation(&resource.DocsConfig{
		OutputDir:      docsDir,
		StackDir:       stackDir,
		StackName:      cfg.Name,
		StackDomain:    cfg.Production.Domain,
		SourceCodeName: cfg.Source.Name,
		SourceCodeURL:  cfg.Source.URL,
	})
	if err != nil {
		return err
	}

	if verbose {
		fmt.Print(content)
	}

	printSuccess(fmt.Sprintf("Documentation generated at %s/README.md", docsDir))
	return nil
}

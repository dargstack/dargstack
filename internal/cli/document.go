package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/resource"
)

var docsCmd = &cobra.Command{
	Use:     "document",
	Aliases: []string{"docs"},
	Short:   "Generate the stack documentation",
	Long: `Generate stack documentation.

Creates a ` + "`README.md`" + ` in the artifacts directory listing all services
found in compose files, along with YAML comments describing each.
Includes a link to the stack domain and source code repository.`,
	RunE: runDocs,
}

func runDocs(_ *cobra.Command, _ []string) error {
	outputDir := cfg.ArtifactsDir()
	docsDir := filepath.Join(outputDir, "docs")

	externalServices := make(map[string]resource.ExternalService, len(cfg.Metadata.ExternalServices))
	for name, svc := range cfg.Metadata.ExternalServices {
		externalServices[name] = resource.ExternalService{
			Description: svc.Description,
		}
	}

	content, err := resource.GenerateDocumentation(&resource.DocsConfig{
		DevDir:           cfg.DevDir(),
		ExternalServices: externalServices,
		OutputDir:        docsDir,
		ProdDir:          cfg.ProdDir(),
		SourceCodeName:   cfg.Metadata.Source.Name,
		SourceCodeURL:    cfg.Metadata.Source.URL,
		StackDomain:      cfg.Environment.Production.Domain,
		StackName:        cfg.Metadata.Name,
	})
	if err != nil {
		return err
	}

	if verbose {
		fmt.Print(content)
	}

	logger.Success(fmt.Sprintf("Documentation generated at %s/README.md", docsDir))
	return nil
}

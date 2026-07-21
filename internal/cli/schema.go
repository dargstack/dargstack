package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/schema"
)

var schemaSave string

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print the dargstack.yaml JSON Schema",
	Long: `Print the JSON Schema for dargstack.yaml to stdout.

The schema can be used for IDE autocomplete and validation.

For IDE integration, save the schema locally and configure your editor:

  dargstack schema --save

Then point your editor's YAML language server to the saved file, or add
a $schema field to your dargstack.yaml:

  $schema: "file:///home/user/.local/share/schemas/dargstack.json"`,
	RunE: runSchema,
}

func init() {
	schemaCmd.Flags().StringVar(&schemaSave, "save", "", "Save schema to a file for IDE integration (default: ~/.local/share/schemas/dargstack.json)")
}

func runSchema(cmd *cobra.Command, _ []string) error {
	raw := schema.Schema()

	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return fmt.Errorf("invalid embedded schema: %w", err)
	}

	if schemaSave != "" {
		if err := os.MkdirAll(filepath.Dir(schemaSave), 0o755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
		f, err := os.Create(schemaSave)
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			return err
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "Schema saved to", schemaSave)
		return nil
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
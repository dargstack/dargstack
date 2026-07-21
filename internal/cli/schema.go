package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/schema"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print the dargstack.yaml JSON Schema",
	Long: `Print the JSON Schema for dargstack.yaml to stdout.

The schema can be used for IDE autocomplete and validation. Add a $schema
field to your dargstack.yaml to enable editor integration:

  $schema: "https://dargstack.io/schema/v4/dargstack.json"`,
	RunE: runSchema,
}

func runSchema(cmd *cobra.Command, _ []string) error {
	raw := schema.Schema()

	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return fmt.Errorf("invalid embedded schema: %w", err)
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSchemaCommandOutput(t *testing.T) {
	cmd := schemaCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("schema command failed: %v", err)
	}

	output := buf.String()
	var v map[string]any
	if err := json.Unmarshal([]byte(output), &v); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}

	if v["title"] != "dargstack.yaml" {
		t.Errorf("expected title=dargstack.yaml, got %v", v["title"])
	}
}

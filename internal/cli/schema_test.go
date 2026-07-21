package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestSchemaSave(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "schema.json")

	cmd := schemaCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.ParseFlags([]string{"--save", savePath})
	if err != nil {
		t.Fatalf("parse flags failed: %v", err)
	}
	cmd.SetArgs([]string{})

	err = cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("schema command failed: %v", err)
	}

	data, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatalf("failed to read saved schema: %v", err)
	}

	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("saved schema is not valid JSON: %v", err)
	}

	if v["title"] != "dargstack.yaml" {
		t.Errorf("expected title=dargstack.yaml, got %v", v["title"])
	}
}

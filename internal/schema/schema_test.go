package schema

import (
	"encoding/json"
	"testing"
)

func TestSchemaIsValidJSON(t *testing.T) {
	raw := Schema()
	var v map[string]any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
}

func TestSchemaHasRequiredKeys(t *testing.T) {
	raw := Schema()
	var v map[string]any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}

	required := []string{"$schema", "$id", "title", "type", "properties"}
	for _, key := range required {
		if _, ok := v[key]; !ok {
			t.Errorf("schema missing required key %q", key)
		}
	}
}

func TestSchemaHasConfigProperties(t *testing.T) {
	raw := Schema()
	var v map[string]any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}

	props, ok := v["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties is not an object")
	}

	expectedProps := []string{"$schema", "metadata", "runtime", "environment"}
	for _, prop := range expectedProps {
		if _, ok := props[prop]; !ok {
			t.Errorf("schema missing property %q", prop)
		}
	}
}

func TestSchemaVersion(t *testing.T) {
	if got := SchemaVersion(); got != "v4" {
		t.Errorf("expected v4, got %s", got)
	}
}

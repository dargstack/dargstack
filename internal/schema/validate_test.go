package schema

import (
	"testing"
)

func TestValidateYAMLEmpty(t *testing.T) {
	if err := ValidateYAML([]byte("{}")); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateYAMLValid(t *testing.T) {
	data := []byte(`
metadata:
  name: teststack
runtime:
  sudo: auto
environment:
  development:
    domain: app.localhost
`)
	if err := ValidateYAML(data); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateYAMLUnknownKey(t *testing.T) {
	data := []byte(`
metadata:
  name: teststack
unknown_key: value
`)
	err := ValidateYAML(data)
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if !isSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T", err)
	}
}

func TestValidateYAMLWrongType(t *testing.T) {
	data := []byte(`
metadata: "not an object"
`)
	err := ValidateYAML(data)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}

func TestValidateYAMLInvalidEnum(t *testing.T) {
	data := []byte(`
runtime:
  sudo: invalid
`)
	err := ValidateYAML(data)
	if err == nil {
		t.Fatal("expected error for invalid enum")
	}
}

func TestValidateYAMLUnknownNestedKey(t *testing.T) {
	data := []byte(`
metadata:
  name: teststack
  unknown_field: value
`)
	err := ValidateYAML(data)
	if err == nil {
		t.Fatal("expected error for unknown nested key")
	}
}

func TestValidateYAMLInvalidYAML(t *testing.T) {
	err := ValidateYAML([]byte("::: invalid"))
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestValidateYAMLSchemaField(t *testing.T) {
	data := []byte(`
$schema: "https://dargstack.io/schema/v4/dargstack.json"
metadata:
  name: teststack
`)
	if err := ValidateYAML(data); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func isSchemaError(err error) bool {
	_, ok := err.(*SchemaError)
	return ok
}

package schema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.yaml.in/yaml/v3"
)

var compiledSchema = func() *jsonschema.Schema {
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema", strings.NewReader(Schema())); err != nil {
		panic(fmt.Sprintf("load schema: %v", err))
	}
	s, err := c.Compile("schema")
	if err != nil {
		panic(fmt.Sprintf("compile schema: %v", err))
	}
	return s
}()

func ValidateYAML(data []byte) error {
	var yamlData map[string]any
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	if yamlData == nil {
		yamlData = map[string]any{}
	}

	jsonData, err := json.Marshal(yamlData)
	if err != nil {
		return fmt.Errorf("convert to json: %w", err)
	}

	var v any
	if err := json.Unmarshal(jsonData, &v); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}

	errs := compiledSchema.Validate(v)
	if errs == nil {
		return nil
	}

	return &SchemaError{detail: formatErrors(errs, "")}
}

type SchemaError struct {
	detail string
}

func (e *SchemaError) Error() string { return e.detail }

func formatErrors(errs error, prefix string) string {
	ve, ok := errs.(*jsonschema.ValidationError)
	if !ok {
		return errs.Error()
	}
	lines := flattenValidationErrors(nil, ve, prefix)
	return strings.Join(lines, "\n")
}

func flattenValidationErrors(lines []string, err *jsonschema.ValidationError, prefix string) []string {
	lines = appendValidationErrors(lines, err, prefix)
	for _, sub := range err.Causes {
		lines = flattenValidationErrors(lines, sub, prefix)
	}
	return lines
}

func appendValidationErrors(lines []string, err *jsonschema.ValidationError, prefix string) []string {
	loc := prefix
	if err.InstanceLocation != "" {
		loc = prefix + err.InstanceLocation
	}
	msg := strings.TrimSpace(strings.TrimPrefix(err.Message, err.InstanceLocation))
	loc = strings.TrimPrefix(loc, "/")
	if loc == "" {
		lines = append(lines, msg)
		return lines
	}
	lines = append(lines, fmt.Sprintf("%s: %s", loc, msg))
	return lines
}

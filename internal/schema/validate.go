package schema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

var compiledSchema = func() *jsonschema.Schema {
	c := jsonschema.NewCompiler()
	var schemaDoc any
	if err := json.Unmarshal([]byte(Schema()), &schemaDoc); err != nil {
		panic(fmt.Sprintf("parse schema: %v", err))
	}
	if err := c.AddResource("schema", schemaDoc); err != nil {
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
	lines := flattenOutputUnits(nil, ve.DetailedOutput().Errors, prefix)
	return strings.Join(lines, "\n")
}

func flattenOutputUnits(lines []string, units []jsonschema.OutputUnit, prefix string) []string {
	for _, unit := range units {
		lines = appendOutputUnit(lines, &unit, prefix)
		lines = flattenOutputUnits(lines, unit.Errors, prefix)
	}
	return lines
}

func appendOutputUnit(lines []string, unit *jsonschema.OutputUnit, prefix string) []string {
	if unit.Error == nil {
		return lines
	}
	loc := prefix
	if unit.InstanceLocation != "" {
		loc = prefix + unit.InstanceLocation
	}
	msg := unit.Error.String()
	loc = strings.TrimPrefix(loc, "/")
	if loc == "" {
		lines = append(lines, msg)
		return lines
	}
	lines = append(lines, fmt.Sprintf("%s: %s", loc, msg))
	return lines
}

package secret

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dargstack/dargstack/v4/internal/compose"

	"go.yaml.in/yaml/v3"
)

// extractDargstackSection extracts a named subsection of x-dargstack (e.g. "secrets" or "configs") from compose data into a map of Template definitions. label is used only to identify the resource kind in error messages.
func extractDargstackSection(composeData []byte, section, label string) (map[string]Template, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", compose.ErrParseCompose, err)
	}

	result := make(map[string]Template)

	raw, ok := doc["x-dargstack"]
	if !ok {
		return result, nil
	}

	ext, ok := raw.(map[string]interface{})
	if !ok {
		return result, nil
	}

	sectionRaw, ok := ext[section]
	if !ok {
		return result, nil
	}

	sectionMap, ok := sectionRaw.(map[string]interface{})
	if !ok {
		return result, nil
	}

	for name, def := range sectionMap {
		// Re-marshal and unmarshal through yaml for clean parsing
		data, err := yaml.Marshal(def)
		if err != nil {
			return nil, fmt.Errorf("%s %q: marshal definition: %w", label, name, err)
		}
		var tmpl Template
		if err := yaml.Unmarshal(data, &tmpl); err != nil {
			return nil, fmt.Errorf("%s %q: parse definition: %w", label, name, err)
		}
		normalizeTemplate(&tmpl)
		result[name] = tmpl
	}

	return result, nil
}

// extractResourcePaths extracts file: paths from a top-level compose resource
// section (e.g. "secrets" or "configs").
func extractResourcePaths(composeData []byte, section string) map[string]string {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil
	}

	resources, ok := doc[section].(map[string]interface{})
	if !ok {
		return nil
	}

	paths := make(map[string]string)
	for name, def := range resources {
		defMap, ok := def.(map[string]interface{})
		if !ok {
			continue
		}
		if filePath, ok := defMap["file"].(string); ok {
			paths[name] = filePath
		}
	}
	return paths
}

// rewriteResourceFilePaths rewrites every <section>.NAME.file: entry in composeData to point to dir/NAME (flat hierarchy).
// The returned bytes are the modified compose document; all existing file: values are replaced regardless of their original path.
// label identifies the resource kind in error messages.
func rewriteResourceFilePaths(composeData []byte, dir, section, label string) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("parse compose for %s path rewrite: %w", label, err)
	}

	sectionRaw, ok := doc[section]
	if !ok {
		return composeData, nil
	}
	sectionMap, ok := sectionRaw.(map[string]interface{})
	if !ok {
		return composeData, nil
	}

	for name, def := range sectionMap {
		defMap, ok := def.(map[string]interface{})
		if !ok {
			continue
		}
		if _, hasFile := defMap["file"]; hasFile {
			defMap["file"] = filepath.Join(dir, name)
			sectionMap[name] = defMap
		}
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("serialize compose after %s path rewrite: %w", label, err)
	}
	return out, nil
}

// writeResourceFiles writes resolved values to their compose-declared file paths with the given file mode. label identifies the resource kind in error messages.
func writeResourceFiles(paths, values map[string]string, mode os.FileMode, label string) error {
	for name, value := range values {
		path, ok := paths[name]
		if !ok {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(value+"\n"), mode); err != nil {
			return fmt.Errorf("write %s %s: %w", label, name, err)
		}
	}
	return nil
}

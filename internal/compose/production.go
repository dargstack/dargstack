package compose

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// StripDevOnlyMarkers removes YAML keys annotated with # dargstack:dev-only comments.
// This processes the raw YAML text before parsing.
func StripDevOnlyMarkers(data []byte) []byte {
	var result []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024) // handle large YAML lines
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "# dargstack:dev-only") {
			continue
		}
		result = append(result, line)
	}
	if scanner.Err() != nil {
		// Fall back to original data rather than returning truncated output.
		return data
	}
	return []byte(strings.Join(result, "\n") + "\n")
}

// StripProductionDevelopmentLabels strips `dargstack.development.*` labels from deploy.labels.
// This is used for production deployments where development-only metadata should
// not reach the deployed stack. Other labels (e.g. dargstack.profiles) are preserved.
func StripProductionDevelopmentLabels(data []byte) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return data, err
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return data, nil
	}

	for _, def := range svcMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}

		deploy, ok := svc["deploy"].(map[string]interface{})
		if !ok {
			continue
		}
		labels, ok := deploy["labels"]
		if !ok {
			continue
		}
		switch v := labels.(type) {
		case map[string]interface{}:
			for k := range v {
				if strings.HasPrefix(k, "dargstack.development.") {
					delete(v, k)
				}
			}
		case []interface{}:
			var filtered []interface{}
			for _, item := range v {
				s, ok := item.(string)
				if ok && strings.HasPrefix(s, "dargstack.development.") {
					continue
				}
				filtered = append(filtered, item)
			}
			deploy["labels"] = filtered
		}
	}

	return yaml.Marshal(doc)
}

// RewriteProductionBindMounts rewrites bind-mount host paths from development
// sources to mirrored production sources when the production path exists.
//
// Rules:
// - Named/pure Docker volumes are unchanged.
// - Relative paths are already resolved to absolute by resolveFilePaths.
// - Only bind sources under devRoot are considered for remapping.
func RewriteProductionBindMounts(data []byte, devRoot, prodRoot string) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return data, err
	}

	services, ok := doc["services"].(map[string]interface{})
	if !ok {
		return data, nil
	}

	for _, def := range services {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}

		vols, ok := svc["volumes"].([]interface{})
		if !ok {
			continue
		}

		for i, raw := range vols {
			switch v := raw.(type) {
			case string:
				host, remainder := splitVolumeSpec(v)
				if remainder == "" {
					continue
				}
				if !filepath.IsAbs(host) {
					continue
				}
				rel, err := filepath.Rel(devRoot, host)
				if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
					continue
				}
				candidate := filepath.Join(prodRoot, rel)
				if _, err := os.Stat(candidate); err == nil {
					vols[i] = candidate + ":" + remainder
				}

			case map[string]interface{}:
				volType, _ := v["type"].(string)
				if volType != "bind" {
					continue
				}
				source, _ := v["source"].(string)
				if !filepath.IsAbs(source) {
					continue
				}
				rel, err := filepath.Rel(devRoot, source)
				if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
					continue
				}
				candidate := filepath.Join(prodRoot, rel)
				if _, err := os.Stat(candidate); err == nil {
					v["source"] = candidate
				}
			}
		}
	}

	return yaml.Marshal(doc)
}

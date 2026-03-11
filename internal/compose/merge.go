package compose

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geofffranks/spruce"
	yamlv2 "gopkg.in/yaml.v2"
	"gopkg.in/yaml.v3"
)

// MergeFiles merges multiple compose YAML files using spruce.
// Later files override earlier ones. Spruce operators like (( prune )) are evaluated.
func MergeFiles(paths ...string) ([]byte, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no compose files provided")
	}

	docs := make([]map[interface{}]interface{}, 0, len(paths))

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}

		// Use yaml.v2 for spruce compatibility (spruce traverses map[interface{}]interface{})
		var doc map[interface{}]interface{}
		if err := yamlv2.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}

		// Resolve relative file: paths to absolute using the compose file's directory.
		// After merge, Docker receives these via stdin and cannot resolve relative paths.
		resolveFilePaths(doc, filepath.Dir(p))

		docs = append(docs, doc)
	}

	// Spruce deep merge: later documents override earlier ones.
	merged, err := spruce.Merge(docs...)
	if err != nil {
		return nil, fmt.Errorf("merge compose files: %w", err)
	}

	// Evaluate spruce operators (prune, grab, etc.)
	ev := &spruce.Evaluator{Tree: merged}
	if err := ev.Run(nil, nil); err != nil {
		return nil, fmt.Errorf("evaluate spruce operators: %w", err)
	}

	// Marshal with yaml.v2 to keep map[interface{}]interface{} roundtrip intact,
	// then re-parse with v3 for clean output formatting.
	v2out, err := yamlv2.Marshal(ev.Tree)
	if err != nil {
		return nil, fmt.Errorf("serialize merged compose: %w", err)
	}

	// Re-marshal through yaml.v3 for consistent output style
	var v3doc interface{}
	if err := yaml.Unmarshal(v2out, &v3doc); err != nil {
		return nil, fmt.Errorf("re-parse merged compose: %w", err)
	}
	result, err := yaml.Marshal(v3doc)
	if err != nil {
		return nil, fmt.Errorf("format merged compose: %w", err)
	}

	return result, nil
}

// LoadSingle loads a single compose file without merging, resolving relative paths.
func LoadSingle(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	// Parse with v2 to resolve paths, then re-marshal
	var v2doc map[interface{}]interface{}
	if err := yamlv2.Unmarshal(data, &v2doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	resolveFilePaths(v2doc, filepath.Dir(path))

	v2out, err := yamlv2.Marshal(v2doc)
	if err != nil {
		return nil, fmt.Errorf("serialize %s: %w", path, err)
	}

	var v3doc interface{}
	if err := yaml.Unmarshal(v2out, &v3doc); err != nil {
		return nil, fmt.Errorf("re-parse %s: %w", path, err)
	}
	result, err := yaml.Marshal(v3doc)
	if err != nil {
		return nil, fmt.Errorf("format %s: %w", path, err)
	}

	return result, nil
}

// MergeEnvFiles merges .env files from development and production.
// Production values override development values. Returns merged content.
func MergeEnvFiles(devEnv, prodEnv string) ([]byte, error) {
	env := make(map[string]string)

	if err := loadEnvFile(devEnv, env); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read dev env: %w", err)
	}

	if err := loadEnvFile(prodEnv, env); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read prod env: %w", err)
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, env[k])
	}

	return []byte(b.String()), nil
}

func loadEnvFile(path string, env map[string]string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.IndexByte(line, '='); idx > 0 {
			env[line[:idx]] = line[idx+1:]
		}
	}
	return scanner.Err()
}

// LoadEnvFile reads a .env file and returns key-value pairs.
func LoadEnvFile(path string) (map[string]string, error) {
	env := make(map[string]string)
	err := loadEnvFile(path, env)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return env, nil
}

// FindMissingEnvValues returns keys that have empty values in the env map.
func FindMissingEnvValues(env map[string]string) []string {
	var missing []string
	for k, v := range env {
		if v == "" {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}

// splitVolumeSpec splits a Docker volume short syntax string "HOST:CONTAINER[:MODE]"
// into the host part and the remainder. It correctly handles Windows drive letter
// prefixes (e.g. "C:\path:/container") by treating a single alpha character
// followed by a colon and a path separator as a drive prefix rather than as the
// HOST:CONTAINER separator.
func splitVolumeSpec(vol string) (host, rest string) {
	idx := strings.IndexByte(vol, ':')
	if idx < 0 {
		return vol, ""
	}
	// Detect a Windows drive letter: exactly one alpha character before the colon,
	// followed by a backslash or forward slash (e.g. "C:\" or "C:/").
	if idx == 1 {
		ch := vol[0]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			after := vol[idx+1:]
			if len(after) > 0 && (after[0] == '\\' || after[0] == '/') {
				// This is a Windows absolute path; the real separator is the NEXT colon.
				if next := strings.IndexByte(after, ':'); next >= 0 {
					split := idx + 1 + next
					return vol[:split], vol[split+1:]
				}
				// No subsequent colon; the whole string is the host with no container.
				return vol, ""
			}
		}
	}
	return vol[:idx], vol[idx+1:]
}

// WriteEnvFile writes key-value pairs to a .env file, sorted by key.
func WriteEnvFile(path string, env map[string]string) error {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, env[k])
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

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

// resolveFilePaths makes relative file: paths in secrets, configs, and bind mounts absolute.
// This is required because after merge, compose is fed to Docker via stdin
// and Docker cannot resolve paths relative to the original compose file.
func resolveFilePaths(doc map[interface{}]interface{}, baseDir string) {
	// Resolve secrets and configs
	for _, section := range []string{"secrets", "configs"} {
		raw, ok := doc[section]
		if !ok {
			continue
		}
		resources, ok := raw.(map[interface{}]interface{})
		if !ok {
			continue
		}
		for _, def := range resources {
			defMap, ok := def.(map[interface{}]interface{})
			if !ok {
				continue
			}
			if filePath, ok := defMap["file"].(string); ok {
				if !filepath.IsAbs(filePath) {
					defMap["file"] = filepath.Join(baseDir, filePath)
				}
			}
		}
	}

	// Resolve volume bind mounts
	servicesRaw, ok := doc["services"]
	if !ok {
		return
	}
	services, ok := servicesRaw.(map[interface{}]interface{})
	if !ok {
		return
	}

	for _, svcDef := range services {
		svc, ok := svcDef.(map[interface{}]interface{})
		if !ok {
			continue
		}
		volumesRaw, ok := svc["volumes"]
		if !ok {
			continue
		}

		volumes, ok := volumesRaw.([]interface{})
		if !ok {
			continue
		}

		for i, volDef := range volumes {
			switch v := volDef.(type) {
			case string:
				// Short syntax: "host:container" or "./relative:container"
				hostPath, remainder := splitVolumeSpec(v)
				if remainder == "" {
					continue
				}
				// Only resolve dot-relative host paths (./ or ../). This preserves
				// named Docker volumes like "pgdata:/var/lib/postgresql/data".
				if strings.HasPrefix(hostPath, ".") {
					absPath := filepath.Join(baseDir, hostPath)
					volumes[i] = absPath + ":" + remainder
				}
			case map[interface{}]interface{}:
				// Long syntax: { type: bind, source: "./path", target: "/container" }
				volType, _ := v["type"].(string)
				if volType != "bind" {
					continue
				}
				source, ok := v["source"].(string)
				if !ok || filepath.IsAbs(source) {
					continue
				}
				// Resolve relative bind mount source
				if strings.HasPrefix(source, ".") || !strings.HasPrefix(source, "/") {
					v["source"] = filepath.Join(baseDir, source)
				}
			}
		}
	}
}

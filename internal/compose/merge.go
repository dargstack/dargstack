package compose

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geofffranks/spruce"
	yamlv2 "gopkg.in/yaml.v2"
	"gopkg.in/yaml.v3"
)

// StackRootPrefix is the "~~" prefix that expands to the stack project root directory.
const StackRootPrefix = "~~"

// PreprocessStackRoot returns a pre-process function that replaces the "~~" stack
// root prefix with the absolute stack directory path in the raw YAML bytes before
// parsing. This is necessary because YAML interprets "~" as null, so "~~" becomes
// the string "~" after parsing. By expanding on raw bytes, we avoid this issue.
func PreprocessStackRoot(stackDir string) func([]byte) []byte {
	return func(data []byte) []byte {
		var result []string
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, StackRootPrefix) {
				line = strings.ReplaceAll(line, StackRootPrefix, stackDir)
			}
			result = append(result, line)
		}
		if scanner.Err() != nil {
			return data
		}
		return []byte(strings.Join(result, "\n") + "\n")
	}
}

// MergeFiles merges multiple compose YAML files using spruce.
// Later files override earlier ones. Spruce operators like (( prune )) are evaluated.
// stackDir is the stack project root directory containing dargstack.yaml, used for
// expanding the "~~" prefix in path values.
func MergeFiles(stackDir string, paths ...string) ([]byte, error) {
	return mergeFiles(nil, stackDir, paths...)
}

// MergeFilesProduction merges compose YAML files like MergeFiles but strips
// # dargstack:dev-only markers from each file's raw bytes before YAML parsing.
// This must be called before the YAML roundtrip, since YAML parsers discard comments.
func MergeFilesProduction(stackDir string, paths ...string) ([]byte, error) {
	return mergeFiles(StripDevOnlyMarkers, stackDir, paths...)
}

// mergeFiles is the shared implementation for MergeFiles and MergeFilesProduction.
// preProcess, when non-nil, is applied to each file's raw bytes before YAML parsing.
// stackDir is the stack project root directory used for expanding the "~~" prefix.
func mergeFiles(preProcess func([]byte) []byte, stackDir string, paths ...string) ([]byte, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no compose files provided")
	}

	// Chain preprocessors: stack root expansion runs first, then any additional pre-processor.
	if stackDir != "" && preProcess != nil {
		stackRootPP := PreprocessStackRoot(stackDir)
		preProcess = func(data []byte) []byte {
			return preProcess(stackRootPP(data))
		}
	} else if stackDir != "" {
		preProcess = PreprocessStackRoot(stackDir)
	}

	docs := make([]map[interface{}]interface{}, 0, len(paths))

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}

		if preProcess != nil {
			data = preProcess(data)
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
// stackDir is the stack project root directory used for expanding the "~~" prefix.
func LoadSingle(stackDir, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if stackDir != "" {
		data = PreprocessStackRoot(stackDir)(data)
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
			if after != "" && (after[0] == '\\' || after[0] == '/') {
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

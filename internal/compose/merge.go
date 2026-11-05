package compose

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geofffranks/spruce"
	"go.yaml.in/yaml/v4"
)

// StackRootPrefix is the "~~" prefix that expands to the stack project root directory.
const StackRootPrefix = "~~"

// ParentDirPrefix is the "~~~" prefix that expands to the parent directory of the stack project root.
const ParentDirPrefix = "~~~"

// PreprocessStackRoot returns a pre-process function that replaces the "~~" stack root prefix with the absolute stack directory path and the "~~~" parent directory prefix with its parent in the raw YAML bytes before parsing.
// This is necessary because YAML interprets "~" as null, so "~~" becomes the string "~" after parsing.
// By expanding on raw bytes, we avoid this issue.
func PreprocessStackRoot(stackDir string) func([]byte) []byte {
	return func(data []byte) []byte {
		parentDir := filepath.Dir(stackDir)
		var result []string
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			// Replace longer prefix first to avoid ~~~ being partially consumed by ~~.
			if strings.Contains(line, ParentDirPrefix) {
				line = strings.ReplaceAll(line, ParentDirPrefix, parentDir)
			}
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

// parseSpruceTree parses a compose document under YAML 1.2, as Docker reads
// it, so unquoted yes/no/on/off/y/n stay strings instead of turning into
// booleans that collide as map keys. The result has the
// map[interface{}]interface{} shape spruce traverses. Duplicate keys are
// allowed with the last one winning, and plain timestamps stay text, both to
// keep existing compose files rendering unchanged. An empty document yields
// a nil map.
func parseSpruceTree(data []byte) (map[interface{}]interface{}, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, err
	}
	if node.Kind == 0 || (node.Kind == yaml.DocumentNode && len(node.Content) == 0) {
		return nil, nil
	}
	keepTimestampsAsText(&node)

	var raw interface{}
	if err := node.Load(&raw, yaml.WithV3Defaults(), yaml.WithUniqueKeys(false)); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	doc, ok := toSpruceTree(raw).(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("expected a mapping at the top level, got %T", raw)
	}
	return doc, nil
}

// keepTimestampsAsText retags plain scalars that resolve to !!timestamp as
// !!str, since decoding them into time.Time would re-render a date such as
// 2001-12-14 as 2001-12-14T00:00:00Z.
func keepTimestampsAsText(n *yaml.Node) {
	if n.Kind == yaml.ScalarNode && n.Style&yaml.TaggedStyle == 0 && n.ShortTag() == "!!timestamp" {
		n.Tag = "!!str"
	}
	for _, c := range n.Content {
		keepTimestampsAsText(c)
	}
}

// toSpruceTree converts the map[string]interface{} maps yaml v4 produces into
// map[interface{}]interface{}, recursing through maps and sequences.
func toSpruceTree(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		m := make(map[interface{}]interface{}, len(t))
		for k, val := range t {
			m[k] = toSpruceTree(val)
		}
		return m
	case map[interface{}]interface{}:
		for k, val := range t {
			t[k] = toSpruceTree(val)
		}
		return t
	case []interface{}:
		for i, val := range t {
			t[i] = toSpruceTree(val)
		}
		return t
	}
	return v
}

// extractPlatformOverlay extracts the x-dargstack.platform.<platform> section from a parsed compose document and returns it as a standalone document with its contents promoted to root level.
// Returns nil if the platform section doesn't exist.
func extractPlatformOverlay(doc map[interface{}]interface{}, platform string) map[interface{}]interface{} {
	xds, ok := doc["x-dargstack"]
	if !ok {
		return nil
	}
	xdsMap, ok := xds.(map[interface{}]interface{})
	if !ok {
		return nil
	}

	platformRaw, ok := xdsMap["platform"]
	if !ok {
		return nil
	}
	platformMap, ok := platformRaw.(map[interface{}]interface{})
	if !ok {
		return nil
	}

	overlayRaw, ok := platformMap[platform]
	if !ok {
		return nil
	}
	overlay, ok := overlayRaw.(map[interface{}]interface{})
	if !ok {
		return nil
	}

	return overlay
}

// MergeFiles merges multiple compose YAML files using spruce.
// Later files override earlier ones.
// Spruce operators like (( prune )) are evaluated.
// stackDir is the stack project root directory used for expanding the "~~" and "~~~" prefixes.
// platform is the target OS (e.g. "darwin", "linux"); if non-empty, x-dargstack.platform.<platform> sections are extracted and merged as higher-priority overlays.
func MergeFiles(stackDir, platform string, paths ...string) ([]byte, error) {
	return mergeFiles(nil, stackDir, platform, paths...)
}

// MergeFilesProduction merges compose YAML files like MergeFiles but strips # dargstack:dev-only markers from each file's raw bytes before YAML parsing.
// platform is the target OS for x-dargstack.platform overrides.
func MergeFilesProduction(stackDir, platform string, paths ...string) ([]byte, error) {
	return mergeFiles(StripDevOnlyMarkers, stackDir, platform, paths...)
}

// mergeFiles is the shared implementation for MergeFiles and MergeFilesProduction.
// preProcess, when non-nil, is applied to each file's raw bytes before YAML parsing.
// stackDir is the stack project root directory used for expanding the "~~" and "~~~" prefixes.
// platform is the target OS for x-dargstack.platform overrides.
func mergeFiles(preProcess func([]byte) []byte, stackDir, platform string, paths ...string) ([]byte, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no compose files provided")
	}

	// Chain preprocessors: stack root expansion runs first, then any additional pre-processor.
	if stackDir != "" && preProcess != nil {
		stackRootPP := PreprocessStackRoot(stackDir)
		inner := preProcess
		preProcess = func(data []byte) []byte {
			return inner(stackRootPP(data))
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

		doc, err := parseSpruceTree(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}

		// Resolve relative file: paths to absolute using the compose file's directory.
		// After merge, Docker receives these via stdin and cannot resolve relative paths.
		resolveFilePaths(doc, filepath.Dir(p))

		docs = append(docs, doc)

		// Inject platform-specific overlay as a higher-priority document.
		if platform != "" {
			if overlay := extractPlatformOverlay(doc, platform); overlay != nil {
				resolveFilePaths(overlay, filepath.Dir(p))
				docs = append(docs, overlay)
			}
		}
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

	// Serialize with yaml v4 rather than yaml.v2 to get its output style.
	result, err := yaml.Marshal(ev.Tree)
	if err != nil {
		return nil, fmt.Errorf("format merged compose: %w", err)
	}

	return result, nil
}

// LoadSingle loads a single compose file without merging, resolving relative paths.
// stackDir is the stack project root directory used for expanding the "~~" and "~~~" prefixes.
// platform is the target OS for x-dargstack.platform overrides.
func LoadSingle(stackDir, platform, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if stackDir != "" {
		data = PreprocessStackRoot(stackDir)(data)
	}

	doc, err := parseSpruceTree(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	resolveFilePaths(doc, filepath.Dir(path))

	// Apply platform overlay if specified.
	if platform != "" {
		if overlay := extractPlatformOverlay(doc, platform); overlay != nil {
			resolveFilePaths(overlay, filepath.Dir(path))
			merged, err := spruce.Merge(doc, overlay)
			if err != nil {
				return nil, fmt.Errorf("merge platform overlay %s: %w", path, err)
			}
			ev := &spruce.Evaluator{Tree: merged}
			if err := ev.Run(nil, nil); err != nil {
				return nil, fmt.Errorf("evaluate platform overlay %s: %w", path, err)
			}
			doc = ev.Tree
		}
	}

	result, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("format %s: %w", path, err)
	}

	return result, nil
}

// splitVolumeSpec splits a Docker volume short syntax string "HOST:CONTAINER[:MODE]" into the host part and the remainder.
// It correctly handles Windows drive letter prefixes (e.g. "C:\path:/container") by treating a single alpha character followed by a colon and a path separator as a drive prefix rather than as the HOST:CONTAINER separator.
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

// resolveFilePaths makes relative file: paths in secrets, configs, bind mounts, and env_file entries absolute.
// This is required because after merge, compose is fed to Docker via stdin and Docker cannot resolve paths relative to the original compose file.
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

		// Resolve env_file entries in string, list, and object forms.
		// Only dot-relative paths are resolved; absolute paths and paths
		// expanded from the stack root prefix are left untouched.
		switch envFileRaw := svc["env_file"].(type) {
		case string:
			if strings.HasPrefix(envFileRaw, ".") {
				svc["env_file"] = filepath.Join(baseDir, envFileRaw)
			}
		case []interface{}:
			for i, entry := range envFileRaw {
				switch e := entry.(type) {
				case string:
					if strings.HasPrefix(e, ".") {
						envFileRaw[i] = filepath.Join(baseDir, e)
					}
				case map[interface{}]interface{}:
					if p, ok := e["path"].(string); ok && strings.HasPrefix(p, ".") {
						e["path"] = filepath.Join(baseDir, p)
					}
				}
			}
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
				// Only resolve dot-relative host paths (./ or ../).
				// This preserves named Docker volumes like "pgdata:/var/lib/postgresql/data".
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

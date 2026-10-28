package secret

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"go.yaml.in/yaml/v3"

	"github.com/dargstack/dargstack/v4/internal/compose"
)

// ConfigHashLen is the number of hex characters of the content hash carried in a config name suffix.
const ConfigHashLen = 8

// configHashSuffixRe matches a config name that carries a content hash suffix: <base>-<8 lowercase hex chars>.
// Adoption of the convention is by suffix detection: only names that already match are policed or pruned.
var configHashSuffixRe = regexp.MustCompile(`^(?P<base>.+)-(?P<hash>[0-9a-f]{8})$`)

// ConfigContentHash returns the first ConfigHashLen hex characters of the SHA-256 of data.
// The hash is taken over the exact bytes, including any trailing newline.
func ConfigContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:ConfigHashLen]
}

// HasConfigHashSuffix reports whether name carries a content hash suffix.
func HasConfigHashSuffix(name string) bool {
	_, _, ok := SplitConfigName(name)
	return ok
}

// SplitConfigName splits a suffixed config name into its base and hash suffix.
// ok is false when the name does not carry a suffix.
func SplitConfigName(name string) (base, hash string, ok bool) {
	m := configHashSuffixRe.FindStringSubmatch(name)
	if m == nil {
		return "", "", false
	}
	return m[configHashSuffixRe.SubexpIndex("base")], m[configHashSuffixRe.SubexpIndex("hash")], true
}

// WithConfigHashSuffix joins a config name base and a content hash suffix.
func WithConfigHashSuffix(base, hash string) string {
	return base + "-" + hash
}

// DerivedConfigHashSuffixes computes the content hash suffix for every derived config (x-dargstack.configs)
// whose name does not already carry one and whose content is available.
// The content is read from the file: path in the compose data; when that file is missing (e.g. a production
// deploy where the source secret is external and the value is not re-derived) it falls back to a leftover
// artifact file from a previous run named <name>-<hash> in the same directory, so the suffix is computed over
// the exact bytes that are about to be deployed.
// Configs whose content is unavailable are left out; the caller can hash nothing for them and deploy validation
// will catch a missing file.
func DerivedConfigHashSuffixes(composeData []byte) (map[string]string, error) {
	templates, err := ExtractConfigTemplates(composeData)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, nil
	}

	paths := ExtractConfigPaths(composeData)
	suffixes := make(map[string]string, len(templates))
	for name := range templates {
		if HasConfigHashSuffix(name) {
			continue
		}
		path, ok := paths[name]
		if !ok {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			data, err = readSuffixedLeftover(filepath.Dir(path), name)
			if err != nil {
				continue
			}
		}
		suffixes[name] = ConfigContentHash(data)
	}
	return suffixes, nil
}

// readSuffixedLeftover reads the single leftover artifact file <dir>/<name>-<hash> from a previous run, if there is exactly one.
func readSuffixedLeftover(dir, name string) ([]byte, error) {
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(name) + `-[0-9a-f]{8}$`)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var left []string
	for _, e := range entries {
		if !e.IsDir() && re.MatchString(e.Name()) {
			left = append(left, e.Name())
		}
	}
	if len(left) != 1 {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(filepath.Join(dir, left[0]))
}

// RenameConfigsWithSuffix renames configs in the in-memory compose document, applying the given name-to-suffix
// mapping to the top-level configs key, every service reference (short and long form), and the rewritten
// artifact file path when the path is derived from the name.
func RenameConfigsWithSuffix(composeData []byte, renames map[string]string) ([]byte, error) {
	if len(renames) == 0 {
		return composeData, nil
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", compose.ErrParseCompose, err)
	}

	if configs, ok := doc["configs"].(map[string]interface{}); ok {
		for name, hash := range renames {
			def, ok := configs[name].(map[string]interface{})
			if !ok {
				continue
			}
			newName := WithConfigHashSuffix(name, hash)
			delete(configs, name)
			configs[newName] = def
			if filePath, ok := def["file"].(string); ok && filepath.Base(filePath) == name {
				def["file"] = filepath.Join(filepath.Dir(filePath), newName)
			}
		}
	}

	if services, ok := doc["services"].(map[string]interface{}); ok {
		for _, def := range services {
			svc, ok := def.(map[string]interface{})
			if !ok {
				continue
			}
			list, ok := svc["configs"].([]interface{})
			if !ok {
				continue
			}
			for i, item := range list {
				switch v := item.(type) {
				case string:
					if hash, ok := renames[v]; ok {
						list[i] = WithConfigHashSuffix(v, hash)
					}
				case map[string]interface{}:
					if src, ok := v["source"].(string); ok {
						if hash, ok := renames[src]; ok {
							v["source"] = WithConfigHashSuffix(src, hash)
						}
					}
				}
			}
		}
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("serialize compose after config suffix rename: %w", err)
	}
	return out, nil
}

// MoveConfigArtifacts renames the on-disk artifact files for renamed configs from <dir>/<name> to <dir>/<name>-<hash>,
// and removes stale leftover artifacts of the same config from previous runs, so the directory mirrors the
// names that are about to be deployed.
// paths maps config name to its pre-rename file: path from the compose data.
func MoveConfigArtifacts(paths, renames map[string]string) (moved []string, err error) {
	if len(renames) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(renames))
	for name := range renames {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		path, ok := paths[name]
		if !ok || filepath.Base(path) != name {
			continue
		}
		dir := filepath.Dir(path)
		newName := WithConfigHashSuffix(name, renames[name])
		newPath := filepath.Join(dir, newName)

		if _, statErr := os.Stat(path); statErr == nil && path != newPath {
			if _, dstErr := os.Stat(newPath); dstErr == nil {
				if rmErr := os.Remove(newPath); rmErr != nil {
					return moved, rmErr
				}
			}
			if err := os.Rename(path, newPath); err != nil {
				return moved, err
			}
			moved = append(moved, newPath)
		}

		// Remove stale leftover artifacts of this config from previous runs (their config objects are pruned by the deploy).
		// The scan is keyed by the base name so it also finds leftovers from a previously rotated name.
		base := name
		if b, _, ok := SplitConfigName(name); ok {
			base = b
		}
		stale, staleErr := findSuffixedFiles(dir, base)
		if staleErr != nil {
			if os.IsNotExist(staleErr) {
				continue
			}
			return moved, staleErr
		}
		for _, f := range stale {
			if f == newPath {
				continue
			}
			if rmErr := os.Remove(f); rmErr != nil {
				return moved, rmErr
			}
		}
	}
	return moved, nil
}

// findSuffixedFiles returns absolute paths of files in dir matching <name>-<8 hex chars>.
func findSuffixedFiles(dir, name string) ([]string, error) {
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(name) + `-[0-9a-f]{8}$`)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && re.MatchString(e.Name()) {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files, nil
}

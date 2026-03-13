package secret

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Template defines secret metadata from x-dargstack.secrets in compose.
type Template struct {
	Hint              string `yaml:"hint"` // human-readable hint for expected value (e.g. for third_party)
	Type              string `yaml:"type"`
	Length            int    `yaml:"length"`             // >0 enables secret generation
	SpecialCharacters *bool  `yaml:"special_characters"` // nil = use default (true), false = opt-out
	Template          string `yaml:"template"`           // template with {{secret_name}} references
	ThirdParty        bool   `yaml:"third_party"`
	InsecureDefault   string `yaml:"insecure_default"`
}

const (
	TypeRandomString  = "random_string"
	TypeWord          = "word"
	TypePrivateKey    = "private_key"
	TypeThirdParty    = "third_party"
	TypeTemplate      = "template"
	TypeInsecureValue = "insecure_default"
)

const thirdPartyPlaceholder = "UNSET THIRD PARTY SECRET"

// ThirdPartyPlaceholder is the placeholder value written to third-party secret files when unset.
const ThirdPartyPlaceholder = thirdPartyPlaceholder

var templateTokenRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// ExtractTemplates extracts x-dargstack.secrets from compose data.
func ExtractTemplates(composeData []byte) (map[string]Template, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("parse compose: %w", err)
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

	secretsRaw, ok := ext["secrets"]
	if !ok {
		return result, nil
	}

	secretsMap, ok := secretsRaw.(map[string]interface{})
	if !ok {
		return result, nil
	}

	for name, def := range secretsMap {
		// Re-marshal and unmarshal through yaml for clean parsing
		data, err := yaml.Marshal(def)
		if err != nil {
			continue
		}
		var tmpl Template
		if err := yaml.Unmarshal(data, &tmpl); err != nil {
			continue
		}
		normalizeTemplate(&tmpl)
		result[name] = tmpl
	}

	return result, nil
}

// ExtractSecretPaths extracts file: paths from the top-level secrets section of compose data.
func ExtractSecretPaths(composeData []byte) map[string]string {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil
	}

	secrets, ok := doc["secrets"].(map[string]interface{})
	if !ok {
		return nil
	}

	paths := make(map[string]string)
	for name, def := range secrets {
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

// RewriteSecretFilePaths rewrites every secrets.NAME.file: entry in composeData to
// point to secretsDir/NAME (flat hierarchy). The returned bytes are the modified compose
// document; all existing file: values are replaced regardless of their original path.
func RewriteSecretFilePaths(composeData []byte, secretsDir string) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("parse compose for secret path rewrite: %w", err)
	}

	secretsRaw, ok := doc["secrets"]
	if !ok {
		return composeData, nil
	}
	secretsMap, ok := secretsRaw.(map[string]interface{})
	if !ok {
		return composeData, nil
	}

	for name, def := range secretsMap {
		defMap, ok := def.(map[string]interface{})
		if !ok {
			continue
		}
		if _, hasFile := defMap["file"]; hasFile {
			defMap["file"] = filepath.Join(secretsDir, name)
			secretsMap[name] = defMap
		}
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("serialize compose after secret path rewrite: %w", err)
	}
	return out, nil
}

// ReadSecretValues reads existing secret values from their compose-declared file paths.
// Placeholder value (UNSET THIRD PARTY SECRET) is excluded so callers
// can treat them as missing and write real values.
func ReadSecretValues(secretPaths map[string]string) map[string]string {
	values := make(map[string]string)
	for name, path := range secretPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		v := strings.TrimSpace(string(data))
		if v != "" && !isPlaceholderValue(v) {
			values[name] = v
		}
	}
	return values
}

// WriteSecrets writes resolved secret values to their compose-declared file paths.
func WriteSecrets(secretPaths, values map[string]string) error {
	for name, value := range values {
		path, ok := secretPaths[name]
		if !ok {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
			return fmt.Errorf("write secret %s: %w", name, err)
		}
	}
	return nil
}

// SecretFileExists returns true if the secret file exists (regardless of content).
func SecretFileExists(secretPaths map[string]string, name string) bool {
	path, ok := secretPaths[name]
	if !ok {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// PlaceholderSecrets returns names of secrets whose file still contains a placeholder value.
func PlaceholderSecrets(composeData []byte, _ string) []string {
	paths := ExtractSecretPaths(composeData)
	if len(paths) == 0 {
		return nil
	}
	var names []string
	for name, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		v := strings.TrimSpace(string(data))
		if isPlaceholderValue(v) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func isPlaceholderValue(v string) bool {
	return IsPlaceholderValue(v)
}

// IsPlaceholderValue reports whether a secret value is a known placeholder.
func IsPlaceholderValue(v string) bool {
	return v == thirdPartyPlaceholder
}

// UnresolvedSecrets returns secret names that need interactive input.
func UnresolvedSecrets(templates map[string]Template, values map[string]string) []string {
	sorted, err := TopologicalSort(templates)
	if err != nil {
		return nil
	}

	var unresolved []string
	for _, name := range sorted {
		if v, ok := values[name]; ok && v != "" {
			continue
		}
		tmpl := templates[name]
		normalizeTemplate(&tmpl)
		if IsAutoGeneratable(&tmpl) || tmpl.Type == TypeThirdParty {
			continue // auto-resolvable
		}
		unresolved = append(unresolved, name)
	}
	return unresolved
}

// IsAutoGeneratable reports whether a template can be generated without direct user input.
func IsAutoGeneratable(t *Template) bool {
	if t == nil {
		return false
	}
	normalizeTemplate(t)
	return t.Type == TypeTemplate ||
		t.Type == TypeRandomString ||
		t.Type == TypeWord ||
		t.Type == TypePrivateKey ||
		t.Type == TypeInsecureValue
}

func normalizeTemplate(t *Template) {
	t.Type = strings.ToLower(strings.TrimSpace(t.Type))
	switch t.Type {
	case "random":
		t.Type = TypeRandomString
	case "wordlist", "wordlist_word":
		t.Type = TypeWord
	case "privatekey":
		t.Type = TypePrivateKey
	case "thirdparty":
		t.Type = TypeThirdParty
	case "default":
		t.Type = TypeInsecureValue
	}

	if t.Type == "" {
		switch {
		case t.ThirdParty:
			t.Type = TypeThirdParty
		case t.Template != "":
			t.Type = TypeTemplate
		case t.InsecureDefault != "":
			t.Type = TypeInsecureValue
		case t.Length > 0 || t.SpecialCharacters != nil:
			t.Type = TypeRandomString
		}
	}
	if t.Type == TypeRandomString {
		if t.Length <= 0 {
			t.Length = 32
		}
		if t.SpecialCharacters == nil {
			v := true
			t.SpecialCharacters = &v
		}
	}
}

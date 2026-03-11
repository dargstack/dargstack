package secret

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

// TopologicalSort returns secret names ordered so dependencies come before dependents.
// Uses Kahn's algorithm: nodes with no incoming edges are processed first.
func TopologicalSort(templates map[string]Template) ([]string, error) {
	// Build dependency graph: name -> list of secrets it depends on
	deps := make(map[string][]string)
	for name, tmpl := range templates {
		if tmpl.Template != "" {
			deps[name] = extractTemplateRefs(tmpl.Template)
		}
	}

	// Detect references to unknown secrets before sorting.
	var missingRefs []string
	for name, refDeps := range deps {
		for _, dep := range refDeps {
			if _, exists := templates[dep]; !exists {
				missingRefs = append(missingRefs, fmt.Sprintf("%s (references unknown %q)", name, dep))
			}
		}
	}
	if len(missingRefs) > 0 {
		sort.Strings(missingRefs)
		return nil, fmt.Errorf("secret templates reference unknown secrets: %s", strings.Join(missingRefs, "; "))
	}

	// Calculate in-degree (number of dependencies for each node)
	inDegree := make(map[string]int, len(templates))
	for name := range templates {
		inDegree[name] = len(deps[name])
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	var sorted []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)

		// Find nodes that depend on this one
		for name, refDeps := range deps {
			for _, dep := range refDeps {
				if dep == node {
					inDegree[name]--
					if inDegree[name] == 0 {
						queue = append(queue, name)
						sort.Strings(queue)
					}
				}
			}
		}
	}

	if len(sorted) < len(templates) {
		sortedSet := make(map[string]bool, len(sorted))
		for _, name := range sorted {
			sortedSet[name] = true
		}
		var cycle []string
		for name := range templates {
			if !sortedSet[name] {
				cycle = append(cycle, name)
			}
		}
		sort.Strings(cycle)
		return nil, fmt.Errorf("circular dependency detected in secret templates: %s", strings.Join(cycle, ", "))
	}

	return sorted, nil
}

func extractTemplateRefs(tmpl string) []string {
	var refs []string
	matches := templateTokenRegex.FindAllStringSubmatch(tmpl, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		if dep, ok := templateDependency(strings.TrimSpace(m[1])); ok {
			refs = append(refs, dep)
		}
	}
	return refs
}

// Resolve processes all secrets: generates missing, resolves templates.
// Returns the resolved values map.
func Resolve(templates map[string]Template, values map[string]string) (map[string]string, error) {
	if values == nil {
		values = make(map[string]string)
	}

	sorted, err := TopologicalSort(templates)
	if err != nil {
		return nil, err
	}

	for _, name := range sorted {
		// Skip if already has a real value
		if v, ok := values[name]; ok && v != "" && !isPlaceholderValue(v) {
			continue
		}

		tmpl := templates[name]
		normalizeTemplate(&tmpl)

		var value string
		switch tmpl.Type {
		case TypeTemplate:
			value, err = resolveTemplate(tmpl.Template, values)
			if err != nil {
				return nil, fmt.Errorf("resolve template %s: %w", name, err)
			}
		case TypeRandomString:
			length := tmpl.Length
			if length <= 0 {
				length = 32
			}
			value, err = generateRandom(length, specialCharsEnabled(&tmpl))
			if err != nil {
				return nil, fmt.Errorf("generate %s: %w", name, err)
			}
		case TypeWord:
			value, err = generateWord()
			if err != nil {
				return nil, fmt.Errorf("generate word %s: %w", name, err)
			}
		case TypePrivateKey:
			value, err = generatePrivateKey()
			if err != nil {
				return nil, fmt.Errorf("generate private key %s: %w", name, err)
			}
		case TypeInsecureValue:
			value = tmpl.InsecureDefault
		case TypeThirdParty:
			continue
		default:
			continue // needs interactive prompt
		}

		values[name] = value
	}

	return values, nil
}

const thirdPartyPlaceholder = "UNSET THIRD PARTY SECRET"

// ThirdPartyPlaceholder is the placeholder value written to third-party secret files when unset.
const ThirdPartyPlaceholder = thirdPartyPlaceholder

func isPlaceholderValue(v string) bool {
	return IsPlaceholderValue(v)
}

// IsPlaceholderValue reports whether a secret value is a known placeholder.
func IsPlaceholderValue(v string) bool {
	return v == thirdPartyPlaceholder
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

// SecretFileExists returns true if the secret file exists (regardless of content).
func SecretFileExists(secretPaths map[string]string, name string) bool {
	path, ok := secretPaths[name]
	if !ok {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
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

// specialCharsEnabled returns the effective special_characters value (default true).
func specialCharsEnabled(t *Template) bool {
	if t.SpecialCharacters == nil {
		return true
	}
	return *t.SpecialCharacters
}

func templateDependency(token string) (string, bool) {
	token = strings.TrimSpace(token)
	switch {
	case token == "", token == "word", token == "private_key", strings.HasPrefix(token, "random"):
		return "", false
	case strings.HasPrefix(token, "secret:"):
		dep := strings.TrimSpace(strings.TrimPrefix(token, "secret:"))
		if dep == "" {
			return "", false
		}
		return dep, true
	default:
		return token, true
	}
}

func resolveTemplate(tmpl string, values map[string]string) (string, error) {
	matches := templateTokenRegex.FindAllStringSubmatchIndex(tmpl, -1)
	if len(matches) == 0 {
		return tmpl, nil
	}

	var b strings.Builder
	last := 0
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		fullStart, fullEnd := m[0], m[1]
		tokStart, tokEnd := m[2], m[3]
		b.WriteString(tmpl[last:fullStart])
		token := strings.TrimSpace(tmpl[tokStart:tokEnd])
		repl, err := evaluateTemplateToken(token, values)
		if err != nil {
			return "", err
		}
		b.WriteString(repl)
		last = fullEnd
	}
	b.WriteString(tmpl[last:])
	return b.String(), nil
}

func generateRandom(length int, includeSpecial bool) (string, error) {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if includeSpecial {
		charset += "!@#$%^&*()-_=+[]{}:,.?"
	}

	out := make([]byte, length)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		out[i] = charset[n.Int64()]
	}
	return string(out), nil
}

func generateWord() (string, error) {
	adjectives := []string{"amber", "brisk", "calm", "daring", "ember", "frost", "gentle", "hazel", "ivory", "jolly"}
	nouns := []string{"falcon", "harbor", "island", "jungle", "keystone", "lantern", "meadow", "nebula", "orchid", "pioneer"}

	a, err := rand.Int(rand.Reader, big.NewInt(int64(len(adjectives))))
	if err != nil {
		return "", err
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(nouns))))
	if err != nil {
		return "", err
	}
	return adjectives[a.Int64()] + "-" + nouns[n.Int64()], nil
}

func generatePrivateKey() (string, error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", err
	}
	p := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return strings.TrimSpace(string(p)), nil
}

func evaluateTemplateToken(token string, values map[string]string) (string, error) {
	switch {
	case token == "word":
		return generateWord()
	case token == "private_key":
		return generatePrivateKey()
	case strings.HasPrefix(token, "random"):
		return parseRandomToken(token)
	case strings.HasPrefix(token, "secret:"):
		name := strings.TrimSpace(strings.TrimPrefix(token, "secret:"))
		return values[name], nil
	default:
		return values[token], nil
	}
}

func parseRandomToken(token string) (string, error) {
	// token forms: random | random:<len> | random:<len>:<special>
	length := 32
	includeSpecial := false

	parts := strings.Split(token, ":")
	if len(parts) >= 2 && strings.TrimSpace(parts[1]) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return "", fmt.Errorf("invalid random token length %q", parts[1])
		}
		if n > 0 {
			length = n
		}
	}
	if len(parts) >= 3 {
		b, err := strconv.ParseBool(strings.TrimSpace(parts[2]))
		if err != nil {
			return "", fmt.Errorf("invalid random token special flag %q", parts[2])
		}
		includeSpecial = b
	}

	return generateRandom(length, includeSpecial)
}

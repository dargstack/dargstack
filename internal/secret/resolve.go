package secret

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

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

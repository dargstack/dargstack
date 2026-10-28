package compose

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ConfigNames returns the names of all top-level configs in the compose document.
func ConfigNames(composeData []byte) ([]string, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrParseCompose, err)
	}

	configMap, ok := doc["configs"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	names := make([]string, 0, len(configMap))
	for name := range configMap {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// yamlFrame is one mapping key on the indentation stack used by RenameConfigInComposeText.
type yamlFrame struct {
	indent int
	key    string
}

// frameKeys returns the key path of the indentation stack.
func frameKeys(stack []yamlFrame) []string {
	keys := make([]string, len(stack))
	for i, f := range stack {
		keys[i] = f.key
	}
	return keys
}

// RenameConfigInComposeText renames a config in a compose source document: the top-level configs map key and
// every service configs reference (short and long form, block and flow style).
// It works line by line on the raw text so comments (including dargstack:dev-only markers) and formatting
// are preserved; a plain YAML round trip would discard them.
// Returns the modified text and whether anything changed.
func RenameConfigInComposeText(text, oldName, newName string) (string, bool) {
	lines := strings.Split(text, "\n")

	// stack tracks the mapping key path by indentation.
	var stack []yamlFrame
	changed := false

	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "---" {
			continue
		}
		indent := len(line) - len(trimmed)

		if isListItem(trimmed) {
			// A list item ends any deeper mapping, e.g. the target/mode keys of a preceding
			// long-form item, or the rename path check below sees a stale frame and bails.
			for len(stack) > 0 && stack[len(stack)-1].indent > indent {
				stack = stack[:len(stack)-1]
			}
			if off := configItemRenameOffset(trimmed, stack, oldName); off >= 0 {
				lines[i] = renameInLine(line, oldName, newName, indent+off)
				changed = true
			}
			continue
		}

		key, rest, isKey := splitSimpleKey(trimmed)
		if !isKey {
			continue
		}
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		path := frameKeys(stack)

		switch {
		case key == oldName && len(path) == 1 && path[0] == "configs":
			// Top-level config key: rename the key token at the indent position.
			lines[i] = line[:indent] + newName + line[indent+len(key):]
			changed = true
		case key == "configs" && isServicePath(path) && strings.HasPrefix(strings.TrimSpace(rest), "["):
			// Flow-style service config list: configs: [old, other]
			if flow := renameFlowList(line, oldName, newName); flow != line {
				lines[i] = flow
				changed = true
			}
		}
		stack = append(stack, yamlFrame{indent: indent, key: key})
	}

	return strings.Join(lines, "\n"), changed
}

// isListItem reports whether a trimmed line is a YAML list item.
func isListItem(trimmed string) bool {
	return trimmed == "-" || strings.HasPrefix(trimmed, "- ")
}

// configItemRenameOffset returns the offset within the trimmed line (starting with "- ") of the oldName token
// that should be renamed, or -1 when the item does not reference oldName.
// Only list items under a service configs key are considered.
func configItemRenameOffset(trimmed string, stack []yamlFrame, oldName string) int {
	if trimmed == "-" {
		return -1
	}
	path := frameKeys(stack)
	if len(path) != 3 || path[0] != "services" || path[2] != "configs" {
		return -1
	}

	content := strings.TrimPrefix(trimmed, "- ")
	if key, value, isKey := splitSimpleKey(content); isKey && key == "source" {
		if unquote(strings.TrimSpace(value)) != oldName {
			return -1
		}
		// The config name is the value after "source:".
		return len("- ") + len(key) + 1
	}
	if unquote(strings.TrimSpace(stripComment(content))) != oldName {
		return -1
	}
	return 2 // after "- "
}

// renameInLine replaces the first occurrence of oldName at or after startOffset, which must be a whole token.
func renameInLine(line, oldName, newName string, startOffset int) string {
	rest := line[startOffset:]
	idx := strings.Index(rest, oldName)
	if idx < 0 {
		return line
	}
	// Only replace when the match is a whole token, not a substring of a longer name.
	end := idx + len(oldName)
	if end < len(rest) && isNameChar(rest[end]) {
		return line
	}
	return line[:startOffset+idx] + newName + rest[end:]
}

// renameFlowList replaces oldName with newName among the items of an inline list in line, preserving the rest.
// Returns the unchanged line when no item matched.
func renameFlowList(line, oldName, newName string) string {
	start := strings.Index(line, "[")
	end := strings.LastIndex(line, "]")
	if start < 0 || end <= start {
		return line
	}
	inner := line[start+1 : end]
	items := strings.Split(inner, ",")
	renamed := false
	for j, item := range items {
		if unquote(strings.TrimSpace(stripComment(item))) == oldName {
			items[j] = renameInLine(item, oldName, newName, 0)
			renamed = true
		}
	}
	if !renamed {
		return line
	}
	return line[:start+1] + strings.Join(items, ",") + line[end:]
}

func isServicePath(path []string) bool {
	return len(path) == 2 && path[0] == "services"
}

// splitSimpleKey splits a trimmed line into key and rest when it starts with a simple mapping key.
func splitSimpleKey(trimmed string) (key, rest string, ok bool) {
	idx := strings.Index(trimmed, ":")
	if idx <= 0 {
		return "", "", false
	}
	key = trimmed[:idx]
	if strings.ContainsAny(key, " \t\"'{[") {
		return "", "", false
	}
	if idx+1 < len(trimmed) && trimmed[idx+1] != ' ' && trimmed[idx+1] != '\t' {
		return "", "", false
	}
	return key, trimmed[idx+1:], true
}

// stripComment removes a trailing YAML comment (whitespace followed by #).
func stripComment(s string) string {
	for i := 0; i+1 < len(s); i++ {
		if s[i+1] == '#' && (s[i] == ' ' || s[i] == '\t') {
			return s[:i+1]
		}
	}
	return s
}

// unquote strips matching surrounding quotes from a YAML scalar.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// isNameChar reports whether c can appear in a compose resource name, used to keep renames whole-token.
func isNameChar(c byte) bool {
	return c == '-' || c == '_' || c == '.' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

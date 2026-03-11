package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dargstack/dargstack/internal/config"
)

// DocsConfig controls the output of GenerateDocumentation.
type DocsConfig struct {
	OutputDir      string
	StackDir       string
	StackName      string
	StackDomain    string
	SourceCodeName string
	SourceCodeURL  string
}

// GenerateDocumentation generates a markdown documentation file for the stack.
// It reads YAML comments from raw service compose files for service descriptions
// and indicates production-only services.
func GenerateDocumentation(dc *DocsConfig) (string, error) {
	devDir := config.DevDir(dc.StackDir)
	prodDir := config.ProdDir(dc.StackDir)

	devServices := listComposeServices(devDir)
	prodServices := listComposeServices(prodDir)

	// All known service names, sorted.
	nameSet := make(map[string]bool)
	for _, n := range devServices {
		nameSet[n] = true
	}
	for _, n := range prodServices {
		nameSet[n] = true
	}
	names := make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}
	sort.Strings(names)

	// Build a set of dev services for the "production-only" check.
	devSet := make(map[string]bool, len(devServices))
	for _, n := range devServices {
		devSet[n] = true
	}

	// Extract comments from raw compose files (dev first, prod as fallback).
	comments := make(map[string]string, len(names))
	for _, name := range names {
		if c := extractServiceCommentAny(devDir, name); c != "" {
			comments[name] = c
		} else if c := extractServiceCommentAny(prodDir, name); c != "" {
			comments[name] = c
		}
	}

	// Extract profile memberships from deploy.labels.dargstack.profiles.
	profileMap := collectProfiles(devDir, prodDir)

	var b strings.Builder

	// Header with stack name and domain.
	if dc.StackName != "" {
		fmt.Fprintf(&b, "# %s\n\n", dc.StackName)
	} else {
		b.WriteString("# Stack Documentation\n\n")
	}
	fmt.Fprintf(&b, "The Docker stack configuration for [%s](https://%s/).", dc.StackDomain, dc.StackDomain)
	if dc.SourceCodeName != "" && dc.SourceCodeURL != "" {
		fmt.Fprintf(&b, " Related to [%s](%s).", dc.SourceCodeName, dc.SourceCodeURL)
	}
	b.WriteString("\n\n")

	// Profiles
	if len(profileMap) > 0 {
		b.WriteString("## Profiles\n\n")
		profileNames := make([]string, 0, len(profileMap))
		for p := range profileMap {
			profileNames = append(profileNames, p)
		}
		sort.Strings(profileNames)

		for _, profile := range profileNames {
			services := profileMap[profile]
			sort.Strings(services)
			fmt.Fprintf(&b, "### %s\n\n", profile)
			fmt.Fprintf(&b, "Services: %s\n\n", strings.Join(services, ", "))
		}
	}

	// Services
	if len(names) > 0 {
		b.WriteString("## Services\n\n")
		for _, name := range names {
			fmt.Fprintf(&b, "### %s", name)
			if !devSet[name] {
				b.WriteString(" *(production only)*")
			}
			b.WriteString("\n\n")

			if comment, ok := comments[name]; ok && comment != "" {
				b.WriteString(comment)
				b.WriteString("\n\n")
			}
		}
	}

	content := b.String()

	if dc.OutputDir != "" {
		if err := os.MkdirAll(dc.OutputDir, 0o755); err != nil {
			return "", fmt.Errorf("create output directory: %w", err)
		}
		outPath := filepath.Join(dc.OutputDir, "README.md")
		if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write documentation: %w", err)
		}
	}

	return content, nil
}

// listComposeServices parses every compose.yaml under dir's service directories
// and returns all service names found in the services: mapping, sorted and deduplicated.
func listComposeServices(dir string) []string {
	nameSet := make(map[string]bool)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		composePath := filepath.Join(dir, e.Name(), "compose.yaml")
		data, err := os.ReadFile(composePath)
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			continue
		}
		svcMap, ok := doc["services"].(map[string]interface{})
		if !ok {
			continue
		}
		for name := range svcMap {
			nameSet[name] = true
		}
	}
	names := make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// extractServiceCommentAny searches all compose.yaml files under baseDir
// for a service matching the given name and extracts its YAML comment.
func extractServiceCommentAny(baseDir, serviceName string) string {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if c := extractServiceComment(baseDir, e.Name(), serviceName); c != "" {
			return c
		}
	}
	return ""
}

// extractServiceComment reads a raw service compose.yaml and extracts the
// YAML comment associated with the named service using the yaml.v3 Node API.
func extractServiceComment(baseDir, dirName, serviceName string) string {
	composePath := filepath.Join(baseDir, dirName, "compose.yaml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		return ""
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return ""
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return ""
	}

	// Find the "services" key
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "services" {
			continue
		}
		svcMapping := root.Content[i+1]
		if svcMapping.Kind != yaml.MappingNode {
			return ""
		}

		// Find the service by name
		for j := 0; j+1 < len(svcMapping.Content); j += 2 {
			keyNode := svcMapping.Content[j]
			valNode := svcMapping.Content[j+1]
			if keyNode.Value != serviceName {
				continue
			}

			// Check comment on the service key itself
			if c := cleanComment(keyNode.HeadComment); c != "" {
				return c
			}

			// Check comment on the first child key of the service mapping
			if valNode.Kind == yaml.MappingNode && len(valNode.Content) > 0 {
				if c := cleanComment(valNode.Content[0].HeadComment); c != "" {
					return c
				}
			}

			return ""
		}
	}
	return ""
}

// cleanComment strips YAML comment prefixes and returns clean markdown text.
func cleanComment(raw string) string {
	if raw == "" {
		return ""
	}
	lines := strings.Split(raw, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimPrefix(line, "# ")
		line = strings.TrimPrefix(line, "#")
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

// collectProfiles parses all compose.yaml files in the dev and prod directories
// and returns a map of profile name → list of service names using that profile.
// Profiles are defined via deploy.labels.dargstack.profiles (comma-separated).
func collectProfiles(devDir, prodDir string) map[string][]string {
	profileMap := make(map[string][]string)
	collectFromDir(devDir, profileMap)
	collectFromDir(prodDir, profileMap)
	return profileMap
}

// collectFromDir scans all compose.yaml files under baseDir and populates profileMap.
func collectFromDir(baseDir string, profileMap map[string][]string) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		composePath := filepath.Join(baseDir, e.Name(), "compose.yaml")
		data, err := os.ReadFile(composePath)
		if err != nil {
			continue
		}

		var doc map[string]interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			continue
		}
		svcMap, ok := doc["services"].(map[string]interface{})
		if !ok {
			continue
		}

		for svcName, svcVal := range svcMap {
			svcDef, ok := svcVal.(map[string]interface{})
			if !ok {
				continue
			}

			deploy, ok := svcDef["deploy"].(map[string]interface{})
			if !ok {
				continue
			}

			labels, ok := deploy["labels"].(map[string]interface{})
			if !ok {
				continue
			}

			profilesVal, ok := labels["dargstack.profiles"]
			if !ok {
				continue
			}

			profilesStr, ok := profilesVal.(string)
			if !ok {
				continue
			}

			// Parse comma-separated profiles
			parts := strings.Split(profilesStr, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				// Add service to this profile's list if not already there
				if !containsString(profileMap[p], svcName) {
					profileMap[p] = append(profileMap[p], svcName)
				}
			}
		}
	}
}

// containsString checks if a slice contains a string.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

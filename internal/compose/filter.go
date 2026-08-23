package compose

import (
	"errors"
	"fmt"
	"strings"

	"regexp"

	"go.yaml.in/yaml/v3"
)

var templateTokenRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// FilterByProfile filters a compose document using dargstack profile semantics.
// When activeProfiles is nil (no --profiles flag): if any service declares a "default" profile, only "default" services are deployed.
// Otherwise all services are deployed.
// When a "default" profile exists, unlabeled services are only included if profile "unlabeled" is explicitly active.
func FilterByProfile(composeData []byte, activeProfiles []string) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrParseComposeForFiltering, err)
	}

	serviceMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return nil, errors.New(ErrNoServicesSection)
	}

	// Detect whether any service declares profile "default".
	hasDefault := false
	for _, def := range serviceMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}
		for _, p := range extractServiceProfiles(svc) {
			if p == "default" {
				hasDefault = true
				break
			}
		}
		if hasDefault {
			break
		}
	}

	// No profile requested: if default exists, activate it; else deploy all.
	if len(activeProfiles) == 0 {
		if hasDefault {
			activeProfiles = []string{"default"}
		} else {
			return composeData, nil
		}
	}

	activeSet := make(map[string]bool, len(activeProfiles))
	for _, p := range activeProfiles {
		activeSet[p] = true
	}

	includeUnlabeled := !hasDefault || activeSet["unlabeled"]

	filtered := make(map[string]interface{})
	for name, def := range serviceMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			filtered[name] = def
			continue
		}

		svcProfiles := extractServiceProfiles(svc)
		if len(svcProfiles) == 0 {
			if includeUnlabeled {
				// Services without profiles are included unless implicit default-mode is active.
				filtered[name] = def
			}
			continue
		}

		// Include if any service profile matches an active profile
		for _, p := range svcProfiles {
			if activeSet[p] {
				filtered[name] = def
				break
			}
		}
	}
	doc["services"] = filtered

	cleanupResources(doc, filtered)

	result, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrSerializeFilteredCompose, err)
	}
	return result, nil
}

// FilterServices filters a compose document to include only the specified services and their referenced top-level resources (secrets, volumes, networks, configs).
func FilterServices(composeData []byte, services []string) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrParseComposeForFiltering, err)
	}

	serviceMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return nil, errors.New(ErrNoServicesSection)
	}

	serviceSet := make(map[string]bool, len(services))
	for _, s := range services {
		serviceSet[s] = true
	}

	filtered := make(map[string]interface{})
	for name, svc := range serviceMap {
		if serviceSet[name] {
			filtered[name] = svc
		}
	}
	doc["services"] = filtered

	cleanupResources(doc, filtered)

	result, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrSerializeFilteredCompose, err)
	}
	return result, nil
}

// ServiceNames returns the names of all services in the compose document.
func ServiceNames(composeData []byte) ([]string, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("parse compose for service names: %w", err)
	}

	serviceMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	names := make([]string, 0, len(serviceMap))
	for name := range serviceMap {
		names = append(names, name)
	}
	return names, nil
}

func DiscoverProfiles(composeData []byte) ([]string, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, err
	}

	serviceMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	profileSet := make(map[string]bool)
	for _, def := range serviceMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}
		for _, p := range extractServiceProfiles(svc) {
			profileSet[p] = true
		}
	}

	var profiles []string
	for p := range profileSet {
		profiles = append(profiles, p)
	}
	return profiles, nil
}

func extractServiceProfiles(svc map[string]interface{}) []string {
	deploy, ok := svc["deploy"].(map[string]interface{})
	if !ok {
		return nil
	}

	labels, ok := deploy["labels"].(map[string]interface{})
	if !ok {
		// labels can also be a list of "key=value" strings
		if labelList, ok := deploy["labels"].([]interface{}); ok {
			for _, item := range labelList {
				if s, ok := item.(string); ok {
					if strings.HasPrefix(s, "dargstack.profiles=") {
						val := strings.TrimPrefix(s, "dargstack.profiles=")
						return splitProfileLabel(val)
					}
				}
			}
		}
		return nil
	}

	raw, ok := labels["dargstack.profiles"]
	if !ok {
		return nil
	}

	if s, ok := raw.(string); ok {
		return splitProfileLabel(s)
	}
	return nil
}

func splitProfileLabel(val string) []string {
	var profiles []string
	for _, p := range strings.Split(val, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			profiles = append(profiles, p)
		}
	}
	return profiles
}

func cleanupResources(doc, filteredServices map[string]interface{}) {
	usedConfigs := make(map[string]bool)
	usedNetworks := make(map[string]bool)
	usedSecrets := make(map[string]bool)
	usedVolumes := make(map[string]bool)

	for _, svc := range filteredServices {
		svcMap, ok := svc.(map[string]interface{})
		if !ok {
			continue
		}
		collectRefs(svcMap, "configs", usedConfigs)
		collectRefs(svcMap, "networks", usedNetworks)
		collectRefs(svcMap, "secrets", usedSecrets)
		collectVolumeRefs(svcMap, usedVolumes)
	}

	// A derived public_key config (x-dargstack.configs) implicitly depends on its source private_key secret, even when no retained service mounts that secret directly (e.g. a service may mount only the derived public key, never the private key it was derived from).
	// Expand usedSecrets accordingly before the secret filtering passes below.
	resolveConfigSecretDeps(doc, usedConfigs, usedSecrets)

	filterTopLevel(doc, "configs", usedConfigs)
	filterTopLevel(doc, "networks", usedNetworks)

	// Resolve transitive template dependencies before filtering secrets.
	// This ensures secrets referenced via {{secret:name}} in a kept secret's template are also kept in both the top-level secrets: and x-dargstack.secrets.
	resolveTransitiveSecretDeps(doc, usedSecrets)

	filterTopLevel(doc, "secrets", usedSecrets)
	filterTopLevel(doc, "volumes", usedVolumes)

	// Also filter x-dargstack.secrets to only the keys in usedSecrets so that secret template metadata for out-of-profile services is not visible to the secret setup flow.
	filterDargstackSecrets(doc, usedSecrets)

	// Also filter x-dargstack.configs to only the keys in usedConfigs so that derived-config metadata for out-of-profile services doesn't dangle: without this, a config removed from the top-level configs: section (because no retained service uses it) stays in x-dargstack.configs and fails validation as a config with no top-level entry, or as a public_key whose source secret was just filtered out of x-dargstack.secrets above.
	filterDargstackConfigs(doc, usedConfigs)
}

// resolveConfigSecretDeps adds the source secret of each used public_key config (from x-dargstack.configs) to usedSecrets, so the private_key secret it's derived from survives filtering even when no retained service mounts that secret directly.
func resolveConfigSecretDeps(doc map[string]interface{}, usedConfigs, usedSecrets map[string]bool) {
	configsMap, ok := dargstackSection(doc, "configs")
	if !ok {
		return
	}

	for name := range usedConfigs {
		def, ok := configsMap[name].(map[string]interface{})
		if !ok {
			continue
		}
		if source, ok := def["source"].(string); ok && source != "" {
			usedSecrets[source] = true
		}
	}
}

// filterDargstackConfigs removes x-dargstack.configs entries whose names are not present in usedConfigs, mirroring filterDargstackSecrets.
func filterDargstackConfigs(doc map[string]interface{}, usedConfigs map[string]bool) {
	configsMap, ok := dargstackSection(doc, "configs")
	if !ok {
		return
	}

	for name := range configsMap {
		if !usedConfigs[name] {
			delete(configsMap, name)
		}
	}
}

// dargstackSection returns the named subsection (e.g. "secrets", "configs")
// of x-dargstack as a map, if present.
func dargstackSection(doc map[string]interface{}, key string) (map[string]interface{}, bool) {
	ext, ok := doc["x-dargstack"]
	if !ok {
		return nil, false
	}
	extMap, ok := ext.(map[string]interface{})
	if !ok {
		return nil, false
	}
	sectionRaw, ok := extMap[key]
	if !ok {
		return nil, false
	}
	sectionMap, ok := sectionRaw.(map[string]interface{})
	return sectionMap, ok
}

// resolveTransitiveSecretDeps expands usedSecrets to include secrets transitively referenced via {{secret:name}} in x-dargstack.secrets templates.
func resolveTransitiveSecretDeps(doc map[string]interface{}, usedSecrets map[string]bool) {
	secretsMap, ok := dargstackSection(doc, "secrets")
	if !ok {
		return
	}
	expandUsedSecrets(secretsMap, usedSecrets)
}

// filterDargstackSecrets removes x-dargstack.secrets entries whose names are not present in usedSecrets.
// usedSecrets should already be expanded with transitive template dependencies via resolveTransitiveSecretDeps.
func filterDargstackSecrets(doc map[string]interface{}, usedSecrets map[string]bool) {
	secretsMap, ok := dargstackSection(doc, "secrets")
	if !ok {
		return
	}

	for name := range secretsMap {
		if !usedSecrets[name] {
			delete(secretsMap, name)
		}
	}
}

// expandUsedSecrets transitively adds secrets referenced via {{secret:name}} in templates of already-used secrets to the usedSecrets set.
func expandUsedSecrets(secretsMap map[string]interface{}, usedSecrets map[string]bool) {
	changed := true
	for changed {
		changed = false
		for name, def := range secretsMap {
			if !usedSecrets[name] {
				continue
			}
			defMap, ok := def.(map[string]interface{})
			if !ok {
				continue
			}
			for _, dep := range extractDargstackSecretRefs(defMap) {
				if !usedSecrets[dep] {
					usedSecrets[dep] = true
					changed = true
				}
			}
		}
	}
}

// extractDargstackSecretRefs extracts secret names referenced via {{secret:name}} or {{name}} from a secret definition's template field.
// It mirrors the templateDependency logic in the secret package.
func extractDargstackSecretRefs(def map[string]interface{}) []string {
	var tmpl string
	switch v := def["template"].(type) {
	case string:
		tmpl = v
	case nil:
		// No template field; not a template secret.
		return nil
	default:
		return nil
	}

	var refs []string
	matches := templateTokenRegex.FindAllStringSubmatch(tmpl, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		token := strings.TrimSpace(m[1])
		switch {
		case token == "", token == "wordlist_word", token == "private_key", strings.HasPrefix(token, "random_string"):
			// Built-in generators, not secret references.
			continue
		case strings.HasPrefix(token, "secret:"):
			dep := strings.TrimSpace(strings.TrimPrefix(token, "secret:"))
			if dep != "" {
				refs = append(refs, dep)
			}
		default:
			// Bare name references (e.g. {{postgres-password}})
			refs = append(refs, token)
		}
	}
	return refs
}

func collectRefs(svc map[string]interface{}, key string, used map[string]bool) {
	raw, ok := svc[key]
	if !ok {
		return
	}

	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			switch ref := item.(type) {
			case string:
				used[ref] = true
			case map[string]interface{}:
				if source, ok := ref["source"].(string); ok {
					used[source] = true
				}
			}
		}
	case map[string]interface{}:
		for name := range v {
			used[name] = true
		}
	}
}

func collectVolumeRefs(svc map[string]interface{}, used map[string]bool) {
	raw, ok := svc["volumes"]
	if !ok {
		return
	}

	vols, ok := raw.([]interface{})
	if !ok {
		return
	}

	for _, item := range vols {
		switch v := item.(type) {
		case string:
			// Short syntax: "volume_name:/path" or "/host:/path"
			name := extractVolumeName(v)
			if name != "" {
				used[name] = true
			}
		case map[string]interface{}:
			// Long syntax: { type: volume, source: name, target: /path }
			if t, ok := v["type"].(string); ok && t == "volume" {
				if source, ok := v["source"].(string); ok {
					used[source] = true
				}
			}
		}
	}
}

// extractVolumeName extracts a named volume from short volume syntax.
// Returns empty string for bind mounts (paths starting with / or .) and for Windows absolute paths (e.g. C:\path:/container or C:/path:/container).
func extractVolumeName(vol string) string {
	// Find the first colon
	for i, c := range vol {
		if c == ':' {
			name := vol[:i]
			// Bind mounts start with / or .
			if name == "" || name[0] == '/' || name[0] == '.' {
				return ""
			}
			// Windows drive letter: single alpha char before the colon, followed by a path separator: treat the whole thing as a bind mount.
			if len(name) == 1 {
				ch := name[0]
				if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
					after := vol[i+1:]
					if after != "" && (after[0] == '\\' || after[0] == '/') {
						return ""
					}
				}
			}
			return name
		}
	}
	return ""
}

func filterTopLevel(doc map[string]interface{}, key string, used map[string]bool) {
	raw, ok := doc[key]
	if !ok {
		return
	}

	resources, ok := raw.(map[string]interface{})
	if !ok {
		return
	}

	if len(used) == 0 {
		delete(doc, key)
		return
	}

	filtered := make(map[string]interface{})
	for name, val := range resources {
		if used[name] {
			filtered[name] = val
		}
	}

	if len(filtered) == 0 {
		delete(doc, key)
	} else {
		doc[key] = filtered
	}
}

package compose

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// FilterByProfile filters a compose document using dargstack profile semantics.
// When activeProfiles is nil (no --profiles flag): if any service declares a "default"
// profile, only "default" services are deployed. Otherwise all services
// are deployed. When a "default" profile exists, unlabeled services are only
// included if profile "unlabeled" is explicitly active.
func FilterByProfile(composeData []byte, activeProfiles []string) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("parse compose for filtering: %w", err)
	}

	serviceMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("compose file has no services section")
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
		return nil, fmt.Errorf("serialize filtered compose: %w", err)
	}
	return result, nil
}

// FilterServices filters a compose document to include only the specified services
// and their referenced top-level resources (secrets, volumes, networks, configs).
func FilterServices(composeData []byte, services []string) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("parse compose for filtering: %w", err)
	}

	serviceMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("compose file has no services section")
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
		return nil, fmt.Errorf("serialize filtered compose: %w", err)
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
	usedSecrets := make(map[string]bool)
	usedVolumes := make(map[string]bool)
	usedNetworks := make(map[string]bool)
	usedConfigs := make(map[string]bool)

	for _, svc := range filteredServices {
		svcMap, ok := svc.(map[string]interface{})
		if !ok {
			continue
		}
		collectRefs(svcMap, "secrets", usedSecrets)
		collectVolumeRefs(svcMap, usedVolumes)
		collectRefs(svcMap, "networks", usedNetworks)
		collectRefs(svcMap, "configs", usedConfigs)
	}

	filterTopLevel(doc, "secrets", usedSecrets)
	filterTopLevel(doc, "volumes", usedVolumes)
	filterTopLevel(doc, "networks", usedNetworks)
	filterTopLevel(doc, "configs", usedConfigs)

	// Also filter x-dargstack.secrets to only the keys in usedSecrets so that
	// secret template metadata for out-of-profile services is not visible to
	// the secret setup flow.
	filterDargstackSecrets(doc, usedSecrets)
}

// filterDargstackSecrets removes x-dargstack.secrets entries whose names are
// not present in usedSecrets.
func filterDargstackSecrets(doc map[string]interface{}, usedSecrets map[string]bool) {
	ext, ok := doc["x-dargstack"]
	if !ok {
		return
	}
	extMap, ok := ext.(map[string]interface{})
	if !ok {
		return
	}
	secretsRaw, ok := extMap["secrets"]
	if !ok {
		return
	}
	secretsMap, ok := secretsRaw.(map[string]interface{})
	if !ok {
		return
	}
	for name := range secretsMap {
		if !usedSecrets[name] {
			delete(secretsMap, name)
		}
	}
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
// Returns empty string for bind mounts (paths starting with / or .) and for
// Windows absolute paths (e.g. C:\path:/container or C:/path:/container).
func extractVolumeName(vol string) string {
	// Find the first colon
	for i, c := range vol {
		if c == ':' {
			name := vol[:i]
			// Bind mounts start with / or .
			if name == "" || name[0] == '/' || name[0] == '.' {
				return ""
			}
			// Windows drive letter: single alpha char before the colon, followed
			// by a path separator — treat the whole thing as a bind mount.
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

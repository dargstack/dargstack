package cli

import (
	"fmt"
	"sort"

	"github.com/dargstack/dargstack/v4/internal/resource"
	"github.com/dargstack/dargstack/v4/internal/secret"
)

// configSetupFlow derives and writes generated config values declared under
// x-dargstack.configs (currently only the public_key type, sourced from a
// private_key secret). Unlike secrets, this never prompts: derivation is
// deterministic given the source secret, so it either succeeds, is skipped
// because the source isn't resolved yet, or fails outright.
func configSetupFlow(composeData []byte) ([]resource.Issue, error) {
	configTemplates, err := secret.ExtractConfigTemplates(composeData)
	if err != nil {
		return nil, fmt.Errorf("extract config templates: %w", err)
	}
	if len(configTemplates) == 0 {
		return nil, nil
	}

	configPaths := secret.ExtractConfigPaths(composeData)

	secretPaths := secret.ExtractSecretPaths(composeData)
	secretValues := secret.ReadSecretValues(secretPaths)

	values, err := secret.ResolveConfigs(configTemplates, secretValues)
	if err != nil {
		return nil, fmt.Errorf("resolve configs: %w", err)
	}

	if err := secret.WriteConfigs(configPaths, values); err != nil {
		return nil, fmt.Errorf("write configs: %w", err)
	}

	names := make([]string, 0, len(configTemplates))
	for name := range configTemplates {
		names = append(names, name)
	}
	sort.Strings(names)

	var issues []resource.Issue
	for _, name := range names {
		if _, ok := configPaths[name]; !ok {
			issues = append(issues, resource.Issue{
				Severity:    "error",
				Resource:    fmt.Sprintf("config:%s", name),
				Description: "derived config has no top-level configs entry with a file: path",
			})
			continue
		}
		if values[name] == "" {
			issues = append(issues, resource.Issue{
				Severity:    "warning",
				Resource:    fmt.Sprintf("config:%s", name),
				Description: "derived config not set: source secret is not resolved yet",
			})
		}
	}

	return issues, nil
}

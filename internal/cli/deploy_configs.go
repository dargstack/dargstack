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
	if len(configPaths) == 0 {
		return nil, nil
	}

	secretPaths := secret.ExtractSecretPaths(composeData)
	secretValues := secret.ReadSecretValues(secretPaths)

	values, err := secret.ResolveConfigs(configTemplates, secretValues)
	if err != nil {
		return nil, fmt.Errorf("resolve configs: %w", err)
	}

	if err := secret.WriteConfigs(configPaths, values); err != nil {
		return nil, fmt.Errorf("write configs: %w", err)
	}

	var missing []string
	for name := range configTemplates {
		if _, ok := configPaths[name]; !ok {
			continue
		}
		if values[name] == "" {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)

	var issues []resource.Issue
	for _, name := range missing {
		issues = append(issues, resource.Issue{
			Severity:    "warning",
			Resource:    fmt.Sprintf("config:%s", name),
			Description: "derived config not set: source secret is not resolved yet",
		})
	}

	return issues, nil
}

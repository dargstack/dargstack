package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dargstack/dargstack/v4/internal/compose"
	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/resource"
	"github.com/dargstack/dargstack/v4/internal/secret"
)

// configSetupFlow derives and writes generated config values declared under x-dargstack.configs (currently only the public_key type, sourced from a private_key secret).
// Unlike secrets, this never prompts: derivation is deterministic given the source secret, so it either succeeds, is skipped because the source isn't resolved yet, or fails outright.
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

// applyDerivedConfigHashSuffixes extends the content hash suffix convention to derived configs (x-dargstack.configs),
// whose content is only known at deploy time: after configSetupFlow wrote the derived files, the suffix is
// computed over the exact bytes about to be deployed and appended to the config name in the in-memory compose
// (top-level key, service references, and the artifact file path, which is derived from the name).
// The suffix is therefore true by construction: a content change produces a new config object instead of
// failing `docker stack deploy`, and the old object becomes a prunable orphan. No repo-recorded name exists
// for derived configs, so there is nothing for validate to police and no rotate command is needed.
// The same name-to-suffix mapping is applied to fullCompose (the unfiltered copy) so post-deploy orphan
// pruning recognizes every name that will exist after this deploy.
func applyDerivedConfigHashSuffixes(deployed, full []byte) (newDeployed, newFull []byte, err error) {
	suffixes, err := secret.DerivedConfigHashSuffixes(deployed)
	if err != nil {
		return nil, nil, fmt.Errorf("compute derived config hash suffixes: %w", err)
	}
	if len(suffixes) == 0 {
		return deployed, full, nil
	}

	paths := secret.ExtractConfigPaths(deployed)
	newDeployed, err = secret.RenameConfigsWithSuffix(deployed, suffixes)
	if err != nil {
		return nil, nil, err
	}
	newFull, err = secret.RenameConfigsWithSuffix(full, suffixes)
	if err != nil {
		return nil, nil, err
	}
	if _, err := secret.MoveConfigArtifacts(paths, suffixes); err != nil {
		return nil, nil, fmt.Errorf("rename config artifacts: %w", err)
	}

	for _, name := range sortedKeys(suffixes) {
		logger.L.Info(fmt.Sprintf("Derived config %s will deploy as %s (content hash suffix)", name, secret.WithConfigHashSuffix(name, suffixes[name])))
	}
	return newDeployed, newFull, nil
}

// hasDerivedConfigs reports whether the compose declares any x-dargstack.configs entries.
func hasDerivedConfigs(composeData []byte) bool {
	templates, err := secret.ExtractConfigTemplates(composeData)
	return err == nil && len(templates) > 0
}

// pruneConfigOrphans removes config objects in the stack namespace that carry a content hash suffix but are no
// longer declared in the full (unfiltered) compose: after a rotation the previous object is unreferenced and
// would otherwise accumulate, because Docker refuses to remove config objects while any service spec references
// them and rejects in-place content updates.
// Pruning is best-effort cleanup after a successful deploy: failures are logged, not returned.
func pruneConfigOrphans(executor *docker.Executor, fullCompose []byte) {
	declared, err := compose.ConfigNames(fullCompose)
	if err != nil {
		logger.L.Warn(fmt.Sprintf("Skipping config orphan cleanup: %v", err))
		return
	}
	declaredSet := make(map[string]bool, len(declared))
	for _, name := range declared {
		declaredSet[name] = true
	}

	listed, err := docker.ConfigList(executor, cfg.Metadata.Name)
	if err != nil {
		logger.L.Warn(fmt.Sprintf("Skipping config orphan cleanup: %v", err))
		return
	}

	orphans := selectConfigOrphans(listed, declaredSet)
	if len(orphans) == 0 {
		return
	}

	fullNames := make([]string, len(orphans))
	for i, name := range orphans {
		fullNames[i] = cfg.Metadata.Name + "_" + name
	}
	if err := docker.ConfigRemove(executor, fullNames); err != nil {
		logger.L.Warn(fmt.Sprintf("Failed to prune orphaned config(s) %s: %v", strings.Join(orphans, ", "), err))
		return
	}
	for _, name := range orphans {
		logger.L.Info(fmt.Sprintf("Pruned orphaned config: %s", name))
	}
}

// selectConfigOrphans returns the sorted config names that carry a content hash suffix but are not in the
// declared set. Names without a suffix are never pruned: the convention is opt-in per config.
func selectConfigOrphans(listed []string, declared map[string]bool) []string {
	var orphans []string
	for _, name := range listed {
		if declared[name] {
			continue
		}
		if !secret.HasConfigHashSuffix(name) {
			continue
		}
		orphans = append(orphans, name)
	}
	sort.Strings(orphans)
	return orphans
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"

	"github.com/dargstack/dargstack/v4/internal/compose"
	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/secret"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage stack configs",
	Long: `Manage stack configs.

Docker only allows label updates on existing config objects: a content update with the same
name fails the deploy. dargstack works around this with an optional content hash suffix
convention: a config named ` + "`<base>-<hash>`" + ` carries the first 8 hex characters of the
SHA-256 of its file content. When the content changes, the name changes with it, so the
deploy creates a new config object instead of failing, and the old one is pruned afterwards.

The convention is opt-in per config: ` + "`dargstack validate`" + ` only polices names that
already carry a suffix, and ` + "`dargstack deploy`" + ` only prunes suffix-named orphans.

Use 'dargstack config rotate <name>' to recompute the suffix after the config content changed.`,
}

var configRotateCmd = &cobra.Command{
	Use:   "rotate <name>",
	Short: "Recompute a config's content hash suffix",
	Long: `Recompute a config's content hash suffix after its content changed.

Reads the config's file content, computes the first 8 hex characters of its SHA-256, and
renames the config to ` + "`<base>-<hash>`" + ` in every source compose file: the top-level
configs key and all service and shared-definition references. The name may be given with
or without the current suffix.

If the config's file is a dargstack artifact under ` + "`artifacts/configs/`" + ` (a derived
config), the artifact file is renamed to match the new name.`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigRotate,
}

func init() {
	configCmd.AddCommand(configRotateCmd)
}

// configBaseName returns the name without any content hash suffix.
func configBaseName(name string) string {
	if base, _, ok := secret.SplitConfigName(name); ok {
		return base
	}
	return name
}

func runConfigRotate(_ *cobra.Command, args []string) error {
	name := args[0]

	composeData, err := buildComposeData(isProduction())
	if err != nil {
		return wrapWithBugHint(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return wrapWithBugHint(err)
	}
	configMap, ok := doc["configs"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no configs found in compose sources")
	}

	currentName, err := resolveRotateTarget(configMap, name)
	if err != nil {
		return err
	}
	cfgDef, ok := configMap[currentName].(map[string]interface{})
	if !ok {
		return fmt.Errorf("config %q has no definition", currentName)
	}
	if ext, _ := cfgDef["external"].(bool); ext {
		return fmt.Errorf("config %q is external and cannot be rotated", currentName)
	}
	filePath, ok := cfgDef["file"].(string)
	if !ok {
		return fmt.Errorf("config %q has no file: path to hash", currentName)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read config file %s: %w", filePath, err)
	}
	hash := secret.ConfigContentHash(data)
	base := configBaseName(currentName)
	newName := secret.WithConfigHashSuffix(base, hash)

	if newName == currentName {
		logger.Success(fmt.Sprintf("Config %q is already up to date (hash %s)", currentName, hash))
		return nil
	}

	if dryRun {
		files, err := collectRenameFiles(currentName)
		if err != nil {
			return err
		}
		logger.L.Info(fmt.Sprintf("[dry-run] Would rename config %s -> %s", currentName, newName))
		for _, f := range files {
			logger.L.Info(fmt.Sprintf("[dry-run]   %s", relToStack(f)))
		}
		return nil
	}

	updated, err := renameConfigInSources(currentName, newName)
	if err != nil {
		return err
	}
	if len(updated) == 0 {
		return fmt.Errorf("config %q not found in source compose files", currentName)
	}

	if _, err := secret.MoveConfigArtifacts(map[string]string{currentName: filePath}, map[string]string{currentName: hash}); err != nil {
		return fmt.Errorf("rename config artifact: %w", err)
	}

	logger.Success(fmt.Sprintf("Rotated config %s -> %s", currentName, newName))
	for _, f := range updated {
		logger.L.Info(fmt.Sprintf("  %s", relToStack(f)))
	}
	warnCrossEnvironmentConfigDrift(currentName, newName, base, filePath, hash)
	logger.L.Info("Run `dargstack validate` to confirm the change.")
	return nil
}

// resolveRotateTarget finds the config to rotate: an exact match when name carries a suffix,
// otherwise every config whose base name matches.
func resolveRotateTarget(configMap map[string]interface{}, name string) (string, error) {
	if _, _, hasSuffix := secret.SplitConfigName(name); hasSuffix {
		if _, exists := configMap[name]; exists {
			return name, nil
		}
		return "", fmt.Errorf("config %q not found (available: %s)", name, strings.Join(sortedConfigNames(configMap), ", "))
	}

	var candidates []string
	for n := range configMap {
		if configBaseName(n) == name {
			candidates = append(candidates, n)
		}
	}
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("config %q not found (available: %s)", name, strings.Join(sortedConfigNames(configMap), ", "))
	case 1:
		return candidates[0], nil
	default:
		sort.Strings(candidates)
		return "", fmt.Errorf("config %q is ambiguous (matches: %s): pass the full name including its suffix", name, strings.Join(candidates, ", "))
	}
}

func sortedConfigNames(configMap map[string]interface{}) []string {
	names := make([]string, 0, len(configMap))
	for n := range configMap {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// sourceComposeFiles returns the compose source files of both environments, development first.
func sourceComposeFiles() []string {
	var files []string
	for _, dir := range []string{cfg.DevDir(), cfg.ProdDir()} {
		svcFiles, err := config.CollectServiceFiles(dir)
		if err != nil {
			continue
		}
		files = append(files, svcFiles...)
	}
	return files
}

// collectRenameFiles returns the source files that reference name in a config key or service reference position.
func collectRenameFiles(name string) ([]string, error) {
	var files []string
	for _, f := range sourceComposeFiles() {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		if _, changed := compose.RenameConfigInComposeText(string(data), name, "probe"); changed {
			files = append(files, f)
		}
	}
	return files, nil
}

// renameConfigInSources renames the config in every source file that references it and returns the updated files.
func renameConfigInSources(oldName, newName string) ([]string, error) {
	var updated []string
	for _, f := range sourceComposeFiles() {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		out, changed := compose.RenameConfigInComposeText(string(data), oldName, newName)
		if !changed {
			continue
		}
		if err := os.WriteFile(f, []byte(out), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", f, err)
		}
		updated = append(updated, f)
	}
	return updated, nil
}

// warnCrossEnvironmentConfigDrift warns when the other environment's content for the same config hashes differently,
// which a single name cannot represent: each environment needs its own config name.
func warnCrossEnvironmentConfigDrift(currentName, newName, base, activePath, activeHash string) {
	otherProd := !isProduction()
	otherData, err := buildComposeData(otherProd)
	if err != nil {
		return
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(otherData, &doc); err != nil {
		return
	}
	configMap, ok := doc["configs"].(map[string]interface{})
	if !ok {
		return
	}
	otherDef, ok := configMap[currentName].(map[string]interface{})
	if !ok {
		return
	}
	otherPath, ok := otherDef["file"].(string)
	if !ok || otherPath == activePath {
		return
	}
	otherBytes, err := os.ReadFile(otherPath)
	if err != nil {
		return
	}
	otherHash := secret.ConfigContentHash(otherBytes)
	if otherHash == activeHash {
		return
	}
	env := "development"
	if otherProd {
		env = "production"
	}
	logger.L.Warn(fmt.Sprintf(
		"the %s content of this config hashes to %s, but %s carries %s. A single name can only carry one content hash: "+
			"if the environments need different content, declare separate config names per environment (base name %q) and reference the right one in each.",
		env, otherHash, newName, activeHash, base,
	))
}

// relToStack shortens a path relative to the stack directory for display.
func relToStack(path string) string {
	if rel, err := filepath.Rel(stackDir, path); err == nil {
		return rel
	}
	return path
}

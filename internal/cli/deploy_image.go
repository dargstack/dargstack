package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/prompt"
)

func resolveDeployTag() (string, error) {
	if deployTag != "" {
		return deployTag, nil
	}
	if cfg.Production.Tag != "latest" {
		return cfg.Production.Tag, nil
	}
	tag, err := latestGitTag(cfg.Production.Branch)
	if err != nil {
		return "", fmt.Errorf("resolve deploy tag from branch %q: %w — use --tag to set explicitly", cfg.Production.Branch, err)
	}
	return tag, nil
}

func latestGitTag(branch string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", branch)
	cmd.Dir = stackDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// autoBuildServices builds images for services that have a dargstack.development.build label.
// Unless behavior.build.skip is true, images are always built (ensuring up-to-date images).
// When skip is true, images are only built if they don't already exist locally.
func autoBuildServices(executor *docker.Executor, composeData []byte) error {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return err
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return nil
	}

	baseDir := config.DevDir(stackDir)

	for name, def := range svcMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}

		contextPath := extractDargstackBuildContext(svc)
		if contextPath == "" {
			continue
		}

		if !filepath.IsAbs(contextPath) {
			// Context paths are relative to the service directory, not the development root.
			// Match the service name to its directory.
			svcDir := filepath.Join(baseDir, name)
			if _, err := os.Stat(svcDir); os.IsNotExist(err) {
				// Service directory doesn't match name — try to find it.
				// For now, assume service name matches directory name.
				printWarning(fmt.Sprintf("Service %q: directory not found at %s", name, svcDir))
				continue
			}
			contextPath = filepath.Join(svcDir, contextPath)
		}

		tag := fmt.Sprintf("%s/%s:development", cfg.Name, name)

		// Skip building if behavior.build.skip is enabled and image already exists.
		if cfg.Behavior.Build != nil && cfg.Behavior.Build.Skip && imageExists(executor, tag) {
			continue
		}

		printInfo(fmt.Sprintf("Auto-building %s", tag))
		if err := docker.StackBuild(executor, contextPath, "development", tag); err != nil {
			return fmt.Errorf("build %s: %w", name, err)
		}
		printSuccess(fmt.Sprintf("Built %s", tag))
	}

	return nil
}

func imageExists(executor *docker.Executor, tag string) bool {
	_, err := executor.Run("image", "inspect", "--format", "{{.ID}}", tag)
	return err == nil
}

// extractDargstackBuildContext returns the build context from a
// deploy.labels.dargstack.development.build label, or "" if not present.
func extractDargstackBuildContext(svc map[string]interface{}) string {
	deploy, ok := svc["deploy"].(map[string]interface{})
	if !ok {
		return ""
	}
	labels, ok := deploy["labels"]
	if !ok {
		return ""
	}
	switch v := labels.(type) {
	case map[string]interface{}:
		if ctx, ok := v["dargstack.development.build"].(string); ok {
			return ctx
		}
	case []interface{}:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			if strings.HasPrefix(s, "dargstack.development.build=") {
				return strings.TrimPrefix(s, "dargstack.development.build=")
			}
		}
	}
	return ""
}

// offerRuntimeCleanup prompts to remove stopped containers and then unused images.
func offerRuntimeCleanup(executor *docker.Executor) {
	ok, err := prompt.Confirm("Remove stopped containers and unused images now?", false)
	if err != nil || !ok {
		return
	}

	containerOut, err := executor.Run("container", "prune", "-f")
	if err != nil {
		printWarning(fmt.Sprintf("Container cleanup failed: %v", err))
		return
	}

	imageOut, err := executor.Run("image", "prune", "-af")
	if err != nil {
		printWarning(fmt.Sprintf("Image cleanup failed: %v", err))
		return
	}

	printSuccess(fmt.Sprintf(
		"Cleanup complete. Containers: %s | Images: %s",
		strings.TrimSpace(containerOut),
		strings.TrimSpace(imageOut),
	))
}

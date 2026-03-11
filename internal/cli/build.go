package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/dargstack/dargstack/internal/config"
	"github.com/dargstack/dargstack/internal/docker"
	"github.com/dargstack/dargstack/internal/prompt"
)

var buildCmd = &cobra.Command{
	Use:   "build [service...]",
	Short: "Build development Dockerfiles",
	Long: `Build service Docker images.

Builds Dockerfiles for services with a dargstack.development.build label in their compose definition.
Each service must have a Dockerfile in the build context directory.

Without arguments, lists available services and prompts you to select which to build.
With service names as arguments, builds only those services.

Images are tagged as <stack>/<service>:development.`,
	RunE: runBuild,
}

func runBuild(cmd *cobra.Command, args []string) error {
	executor, err := docker.NewExecutor(cfg.Sudo)
	if err != nil {
		return err
	}

	// Build compose from service files
	composeData, err := buildDevelopmentCompose()
	if err != nil {
		return fmt.Errorf("build compose: %w", err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return fmt.Errorf("parse compose: %w", err)
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no services found in compose")
	}

	// Determine which services to build
	toBuild := args

	if len(toBuild) == 0 {
		toBuild, err = selectBuildableServices(svcMap)
		if err != nil {
			return err
		}
	} else {
		// Validate requested services exist and have build contexts
		for _, name := range toBuild {
			svcDef, ok := svcMap[name].(map[string]interface{})
			if !ok {
				return fmt.Errorf("service %q not found in compose — have you cloned its repository?", name)
			}
			contextPath := extractDargstackBuildContext(svcDef)
			if contextPath == "" {
				return fmt.Errorf("service %q has no dargstack.development.build label — it uses a pre-built image", name)
			}
			if !filepath.IsAbs(contextPath) {
				// Context is relative to the service directory.
				svcDir := filepath.Join(config.DevDir(stackDir), name)
				contextPath = filepath.Join(svcDir, contextPath)
			}
			if _, statErr := os.Stat(contextPath); os.IsNotExist(statErr) {
				return fmt.Errorf("build context for %q not found at %s — have you cloned its repository?", name, contextPath)
			}
		}
	}

	if len(toBuild) == 0 {
		printInfo("No services selected for building")
		return nil
	}

	for _, svcName := range toBuild {
		svcDef := svcMap[svcName].(map[string]interface{})
		contextPath := extractDargstackBuildContext(svcDef)
		if !filepath.IsAbs(contextPath) {
			// Context is relative to the service directory.
			svcDir := filepath.Join(config.DevDir(stackDir), svcName)
			contextPath = filepath.Join(svcDir, contextPath)
		}
		tag := fmt.Sprintf("%s/%s:development", cfg.Name, svcName)

		printInfo(fmt.Sprintf("Building %s from %s", tag, contextPath))
		if err := docker.StackBuild(executor, contextPath, "development", tag); err != nil {
			return fmt.Errorf("build %s: %w", svcName, err)
		}
		printSuccess(fmt.Sprintf("Built %s", tag))
	}

	return nil
}

// selectBuildableServices discovers services with a dargstack.development.build label,
// classifies them as available (context exists) or unavailable, and prompts the user.
func selectBuildableServices(svcMap map[string]interface{}) ([]string, error) {
	var available, unavailable []string

	for name, def := range svcMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}
		contextPath := extractDargstackBuildContext(svc)
		if contextPath == "" {
			continue // no dargstack.development.build label
		}
		if !filepath.IsAbs(contextPath) {
			// Context is relative to the service directory.
			svcDir := filepath.Join(config.DevDir(stackDir), name)
			contextPath = filepath.Join(svcDir, contextPath)
		}
		if _, statErr := os.Stat(contextPath); os.IsNotExist(statErr) {
			unavailable = append(unavailable, name)
		} else {
			available = append(available, name)
		}
	}

	sort.Strings(available)
	sort.Strings(unavailable)

	if len(available) == 0 && len(unavailable) == 0 {
		printInfo("No services have a dargstack.development.build label")
		return nil, nil
	}

	if len(unavailable) > 0 {
		printWarning(fmt.Sprintf("Unavailable for build (context directory not found — clone their repositories): %s",
			joinNames(unavailable)))
	}

	if len(available) == 0 {
		printInfo("No buildable services have their context directory present")
		return nil, nil
	}

	if noInteraction {
		return available, nil
	}

	selected, err := prompt.MultiSelect("Select services to build:", available)
	if err != nil {
		return nil, err
	}
	return selected, nil
}

func joinNames(names []string) string {
	result := ""
	for i, n := range names {
		if i > 0 {
			result += ", "
		}
		result += n
	}
	return result
}

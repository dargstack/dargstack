package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/dargstack/dargstack/v4/internal/compose"
	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/prompt"
)

var buildCmd = &cobra.Command{
	Use:   "build [service...]",
	Short: "Build development Dockerfiles",
	Long: `Build service Docker images.

Builds Dockerfiles for services with a ` + "`dargstack.development.build`" + ` or ` + "`dargstack.development.git`" + ` label in their compose definition.
The ` + "`dargstack.development.build`" + ` label takes precedence over ` + "`dargstack.development.git`" + `.
Each service must have a Dockerfile in the build context directory.

Without arguments, lists available services and prompts you to select which to build.
With service names as arguments, builds only those services.

Images are tagged as ` + "`<stack>/<service>:development`" + `.`,
	RunE: runBuild,
}

func runBuild(cmd *cobra.Command, args []string) error {
	executor, err := docker.NewExecutor(string(cfg.Runtime.Sudo))
	if err != nil {
		return err
	}

	// Build compose from service files
	composeData, err := buildDevelopmentCompose()
	if err != nil {
		return fmt.Errorf("build compose: %w", err)
	}

	// Filter by profile/services so only active services are offered for building.
	if !deployAll {
		switch {
		case len(profiles) > 0:
			composeData, err = compose.FilterByProfile(composeData, profiles)
			if err != nil {
				return fmt.Errorf("filter profiles %v: %w", profiles, err)
			}
			logger.L.Info(fmt.Sprintf("Building with profiles %v active", profiles))
		case len(services) > 0:
			composeData, err = compose.FilterServices(composeData, services)
			if err != nil {
				return fmt.Errorf("filter services: %w", err)
			}
			logger.L.Info(fmt.Sprintf("Building services: %s", joinNames(services)))
		default:
			hasDefault := composeHasProfile(composeData, "default")
			composeData, err = compose.FilterByProfile(composeData, nil)
			if err != nil {
				return fmt.Errorf("apply default profile semantics: %w", err)
			}
			if hasDefault {
				logger.L.Info("Default profile active: only services in profile \"default\" are available for building")
			}
		}
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return fmt.Errorf("%s: %w", compose.ErrParseCompose, err)
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
			contextPath := resolveBuildContext(svcDef, stackDir)
			if contextPath == "" {
				return fmt.Errorf("service %q has no dargstack.development.build or dargstack.development.git label — it uses a pre-built image", name)
			}
			if !filepath.IsAbs(contextPath) {
				// Context is relative to the service directory.
				svcDir := filepath.Join(cfg.DevDir(), name)
				contextPath = filepath.Join(svcDir, contextPath)
			}
			if _, statErr := os.Stat(contextPath); os.IsNotExist(statErr) {
				return fmt.Errorf("build context for %q not found at %s — have you cloned its repository?", name, contextPath)
			}
		}
	}

	if len(toBuild) == 0 {
		logger.L.Info("No services selected for building")
		return nil
	}

	// Resolve build contexts for all services.
	type resolvedBuild struct {
		name        string
		contextPath string
		tag         string
	}
	var builds []resolvedBuild
	for _, svcName := range toBuild {
		svcDef := svcMap[svcName].(map[string]interface{})
		contextPath := resolveBuildContext(svcDef, stackDir)
		if !filepath.IsAbs(contextPath) {
			svcDir := filepath.Join(cfg.DevDir(), svcName)
			contextPath = filepath.Join(svcDir, contextPath)
		}
		tag := fmt.Sprintf("%s/%s:development", cfg.Metadata.Name, svcName)
		builds = append(builds, resolvedBuild{name: svcName, contextPath: contextPath, tag: tag})
	}

	if verbose {
		for _, b := range builds {
			logger.L.Info(fmt.Sprintf("Building %s from %s", b.tag, b.contextPath))
		}
	}

	// Non-verbose: show per-service status lines.
	type buildStatus struct {
		mu     sync.Mutex
		tasks  []resolvedBuild
		status map[string]string
		lines  int
	}

	var bs *buildStatus
	if !verbose {
		bs = &buildStatus{
			tasks:  builds,
			status: make(map[string]string, len(builds)),
		}
		bs.mu.Lock()
		for _, t := range bs.tasks {
			fmt.Printf("  [%s] building...\033[K\n", t.name)
		}
		bs.lines = len(bs.tasks)
		bs.mu.Unlock()
		defer func() {
			bs.mu.Lock()
			for i := 0; i < bs.lines; i++ {
				fmt.Print("\033[2K\r\n")
			}
			fmt.Printf("\033[%dA", bs.lines)
			bs.mu.Unlock()
		}()
	}

	redrawStatus := func() {
		if bs == nil {
			return
		}
		bs.mu.Lock()
		if bs.lines > 0 {
			fmt.Printf("\033[%dA", bs.lines)
		}
		for _, t := range bs.tasks {
			s := bs.status[t.name]
			if s == "" {
				s = "building..."
			}
			fmt.Printf("  [%s] %s\033[K\n", t.name, s)
		}
		bs.mu.Unlock()
	}

	// Run builds in parallel.
	var wg sync.WaitGroup
	var mu sync.Mutex
	var buildErrs []string

	for _, b := range builds {
		wg.Add(1)
		go func(rb resolvedBuild) {
			defer wg.Done()
			if err := docker.StackBuild(executor, rb.name, verbose, rb.contextPath, "development", rb.tag); err != nil {
				if bs != nil {
					bs.mu.Lock()
					bs.status[rb.name] = "✗ failed"
					bs.mu.Unlock()
					redrawStatus()
				}
				mu.Lock()
				buildErrs = append(buildErrs, fmt.Sprintf("build %s: %v", rb.name, err))
				mu.Unlock()
				return
			}
			if bs != nil {
				bs.mu.Lock()
				bs.status[rb.name] = "✓ built"
				bs.mu.Unlock()
				redrawStatus()
			}
		}(b)
	}

	wg.Wait()

	if len(buildErrs) > 0 {
		return fmt.Errorf("build errors:\n  %s", joinNamesWithNewline(buildErrs))
	}

	if verbose {
		for _, b := range builds {
			logger.Success(fmt.Sprintf(MsgBuiltImage, b.tag))
		}
	} else {
		logger.Success(fmt.Sprintf("Built %d image(s)", len(builds)))
	}

	return nil
}

// selectBuildableServices discovers services with a `dargstack.development.build` or
// `dargstack.development.git` label, classifies them as available (context exists)
// or unavailable, and prompts the user.
func selectBuildableServices(svcMap map[string]interface{}) ([]string, error) {
	var available, unavailable []string

	for name, def := range svcMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}
		contextPath := resolveBuildContext(svc, stackDir)
		if contextPath == "" {
			continue // no build or git label
		}
		if !filepath.IsAbs(contextPath) {
			// Context is relative to the service directory.
			svcDir := filepath.Join(cfg.DevDir(), name)
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
		logger.L.Info("No services have a `dargstack.development.build` or `dargstack.development.git` label")
		return nil, nil
	}

	if len(unavailable) > 0 {
		logger.L.Warn(fmt.Sprintf("Unavailable for build (context directory not found — clone their repositories): %s",
			joinNames(unavailable)))
	}

	if len(available) == 0 {
		logger.L.Info("No buildable services have their context directory present")
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

func joinNamesWithNewline(items []string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += "\n  "
		}
		result += item
	}
	return result
}

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/dargstack/dargstack/v4/internal/audit"
	"github.com/dargstack/dargstack/v4/internal/compose"
	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/prompt"
	"github.com/dargstack/dargstack/v4/internal/resource"
	"github.com/dargstack/dargstack/v4/internal/secret"
	"github.com/dargstack/dargstack/v4/internal/tls"
)

func runDeployWithExecutor(ctx context.Context, _ *cobra.Command, dockerClient *docker.Client, executor *docker.Executor, env string, dryRun bool) error {
	composeData, err := buildComposeData(isProduction())
	if err != nil {
		return wrapWithBugHint(err)
	}

	composeVars := applyEnvToProcess(isProduction())

	if dryRun {
		printInfo("[dry-run] Would check for missing environment variable values")
	} else {
		if err := promptForEnvValues(isProduction()); err != nil {
			printWarning(fmt.Sprintf("Environment variable check: %v", err))
		}
		executor.SetComposeEnv(composeVars)
	}

	// Filter for secrets and validation.
	if err := deployFilterForSecrets(composeData, dryRun); err != nil {
		return err
	}

	// Filter for all subsequent operations so TLS, git cloning, repo
	// fetching, and auto-builds only operate on services that will be deployed.
	composeData, filterMsg, filterErr := applyProfileFilter(composeData)
	if filterErr != nil {
		return fmt.Errorf("filter compose by profile: %w", filterErr)
	}
	printInfo(filterMsg)

	if !isProduction() {
		composeData, err = deployPrepareDevelopment(ctx, dockerClient, executor, composeData, dryRun)
		if err != nil {
			return err
		}
	}

	if isProduction() {
		composeData, err = deployPreDeployChecks(executor, composeData, dryRun)
		if err != nil {
			return err
		}
	}

	if err := deployExecute(executor, composeData, env, dryRun); err != nil {
		return err
	}

	deployPostDeploy(ctx, dockerClient, executor, composeData, dryRun)

	return nil
}

// deployFilterForSecrets filters composeData by profile, runs secret setup,
// and validates resources.
func deployFilterForSecrets(composeData []byte, dryRun bool) error {
	secretComposeData, _, filterErr := applyProfileFilter(composeData)
	if filterErr != nil {
		return fmt.Errorf("filter compose by profile for secret setup: %w", filterErr)
	}

	if dryRun {
		templates, templateErr := secret.ExtractTemplates(secretComposeData)
		if templateErr == nil && len(templates) > 0 {
			printInfo(fmt.Sprintf("[dry-run] Would set up %d secret(s):", len(templates)))
			for name, tmpl := range templates {
				switch {
				case tmpl.Type == secret.TypeThirdParty || tmpl.ThirdParty:
					msg := fmt.Sprintf("  %s: third-party (provide manually)", name)
					if tmpl.Hint != "" {
						msg += fmt.Sprintf(" — %s", tmpl.Hint)
					}
					printInfo(msg)
				case tmpl.Type == secret.TypeTemplate || tmpl.Template != "":
					printInfo(fmt.Sprintf("  %s: template", name))
				case tmpl.Type == secret.TypeWordlistWord:
					printInfo(fmt.Sprintf("  %s: generated word", name))
				case tmpl.Type == secret.TypePrivateKey:
					printInfo(fmt.Sprintf("  %s: generated private key", name))
				case tmpl.Type == secret.TypeInsecureDefault:
					printInfo(fmt.Sprintf("  %s: insecure default value", name))
				case secret.IsAutoGeneratable(&tmpl):
					length := tmpl.Length
					if length <= 0 {
						length = 32
					}
					printInfo(fmt.Sprintf("  %s: generated (%d chars)", name, length))
				default:
					printInfo(fmt.Sprintf("  %s: interactive prompt required", name))
				}
			}
		} else {
			printInfo("[dry-run] No secrets to set up")
		}
	} else {
		if err, _ := secretSetupFlow(secretComposeData, isProduction(), true); err != nil {
			return fmt.Errorf("secret setup: %w", err)
		}
	}

	if dryRun {
		printInfo("[dry-run] Would validate all stack resources")
	} else {
		issues, err := resource.Validate(secretComposeData, stackDir, isProduction())
		if err != nil {
			return wrapWithBugHint(err)
		}
		if printIssues(issues) {
			return hintErr(
				errors.New(ErrValidationFailed),
				"Fix the errors listed above, then run `dargstack deploy` again.",
			)
		}
	}

	return nil
}

// deployPrepareDevelopment handles TLS certs, git clones, repo fetches,
// auto-builds, and volume cleanup for development deployments.
func deployPrepareDevelopment(ctx context.Context, dockerClient *docker.Client, executor *docker.Executor, composeData []byte, dryRun bool) ([]byte, error) {
	// TLS certificates
	domains := uniqueSortedDomains(tls.ExtractDomains(composeData, cfg.Development.Domain), cfg.Development.Certificate.Domains)
	if dryRun {
		printInfo(fmt.Sprintf("[dry-run] Would ensure TLS certificates for: %s", strings.Join(domains, ", ")))
	} else {
		certDir := config.CertificatesDir(stackDir)
		if err := tls.EnsureCertificates(certDir, domains); err != nil {
			printWarning(fmt.Sprintf("TLS certificate setup failed: %v", err))
		}
	}

	// Clone git repos
	if dryRun {
		gitServices := extractGitServices(composeData)
		if len(gitServices) > 0 {
			printInfo(fmt.Sprintf("[dry-run] Would clone and initialize repositories for: %s", strings.Join(gitServices, ", ")))
		}
	} else {
		var err error
		composeData, err = cloneGitRepos(stackDir, composeData)
		if err != nil {
			return nil, fmt.Errorf("clone git repos: %w", err)
		}
	}

	// Fetch build-context repos and warn if behind
	if !dryRun {
		behindRepos := fetchAndWarnBehind(composeData)
		printBehindWarning(behindRepos)
	}

	// Auto-build images
	buildServices := extractBuildServices(composeData)
	if dryRun {
		if len(buildServices) > 0 {
			printInfo(fmt.Sprintf("[dry-run] Would auto-build images for: %s", strings.Join(buildServices, ", ")))
		} else {
			printInfo("[dry-run] No services require auto-build")
		}
	} else {
		if err := autoBuildServices(executor, composeData); err != nil {
			return nil, fmt.Errorf("auto-build failed: %w", err)
		}
	}

	// Volume cleanup prompt
	if dryRun {
		printInfo("[dry-run] Would prompt for volume cleanup (first-time deploy)")
	} else if !noInteraction {
		promptVolumes := true
		if cfg.Behavior.Volume != nil && cfg.Behavior.Volume.Remove != nil {
			promptVolumes = cfg.Behavior.Volume.Remove.Prompt
		}
		if promptVolumes {
			running := isStackRunning(ctx, dockerClient, executor)
			if !running {
				ok, _ := prompt.Confirm("Remove all stack volumes for a clean start?", false)
				if ok {
					volumes, volErr := docker.VolumeList(executor, cfg.Name)
					if volErr == nil && len(volumes) > 0 {
						if err := docker.VolumeRemove(executor, volumes); err != nil {
							printWarning(fmt.Sprintf("Failed to remove volumes: %v", err))
						} else {
							for _, v := range volumes {
								printInfo(fmt.Sprintf("  Removed volume: %s", v))
							}
							printSuccess(fmt.Sprintf(MsgRemovedVolumes, len(volumes)))
						}
					}
				}
			}
		}
	}

	return composeData, nil
}

// deployPreDeployChecks validates production image accessibility and strips
// development labels before deployment.
func deployPreDeployChecks(executor *docker.Executor, composeData []byte, dryRun bool) ([]byte, error) {
	images := compose.ExtractServiceImages(composeData)
	if dryRun {
		if len(images) > 0 {
			printInfo(fmt.Sprintf("[dry-run] Would check image accessibility for: %s", strings.Join(images, ", ")))
		}
	} else {
		if len(images) > 0 {
			unreachable := docker.CheckImagesAccessible(executor, images)
			if len(unreachable) > 0 {
				for img := range unreachable {
					printError(fmt.Sprintf("image not accessible: %s", img))
				}
				return nil, fmt.Errorf("one or more images are not accessible — fix registry credentials or image references before deploying")
			}
		}
	}

	var err error
	composeData, err = compose.StripProductionDevelopmentLabels(composeData)
	if err != nil {
		return nil, wrapWithBugHint(err)
	}
	return composeData, nil
}

// deployExecute prints deploy messaging, saves the audit snapshot, and
// runs docker stack deploy.
func deployExecute(executor *docker.Executor, composeData []byte, env string, dryRun bool) error {
	if isProduction() {
		if cfg.Production.Domain == "app.localhost" {
			prefix := ""
			if dryRun {
				prefix = "[dry-run] "
			}
			printWarning(prefix + "STACK_DOMAIN is still set to default \"app.localhost\" — set domain in dargstack.yaml for production")
		}
		tag := "unknown"
		if !dryRun {
			resolvedTag, tagErr := resolveDeployTag()
			if tagErr != nil {
				printWarning(fmt.Sprintf("Deploy tag resolution failed: %v", tagErr))
			} else {
				tag = resolvedTag
			}
		}
		if dryRun {
			printInfo(fmt.Sprintf("[dry-run] Would deploy production stack %q (tag: %s)", cfg.Name, tag))
		} else {
			printInfo(fmt.Sprintf("Deploying production stack %q (tag: %s)", cfg.Name, tag))
		}
	} else {
		if dryRun {
			printInfo(fmt.Sprintf("[dry-run] Would deploy development stack %q", cfg.Name))
		} else {
			printInfo(fmt.Sprintf("Deploying development stack %q", cfg.Name))
		}
	}

	// Save audit trail
	if dryRun {
		printInfo("[dry-run] Would save deployment snapshot to audit log")
	} else {
		auditDir := audit.AuditLogDir(stackDir)
		var auditPath string
		if snapPath, saveErr := audit.SaveDeployment(auditDir, env, composeData); saveErr == nil {
			auditPath = snapPath
			if verbose {
				printInfo(fmt.Sprintf("Deployment snapshot: %s", auditPath))
			}
		} else if verbose {
			printWarning(fmt.Sprintf("Failed to save audit snapshot: %v", saveErr))
		}
	}

	// Execute deployment
	if dryRun {
		printInfo("[dry-run] Would execute `docker stack deploy`")
		printInfo("[dry-run] Final compose output:")
		fmt.Println()
		fmt.Print(string(composeData))
	} else {
		if err := docker.StackDeploy(executor, cfg.Name, composeData); err != nil {
			return wrapWithBugHint(err)
		}
	}

	return nil
}

// deployPostDeploy prints service count and offers runtime cleanup.
func deployPostDeploy(ctx context.Context, dockerClient *docker.Client, executor *docker.Executor, composeData []byte, dryRun bool) {
	if dryRun {
		svcCount := countComposeServices(composeData)
		printInfo(fmt.Sprintf("[dry-run] Would have %d service(s) running", svcCount))
	} else {
		if count, err := countStackServices(ctx, dockerClient, executor); err == nil {
			printSuccess(fmt.Sprintf("Stack %q deployed with %d service(s)", cfg.Name, count))
		}
	}

	if isProduction() {
		if dryRun {
			printInfo("[dry-run] Would offer runtime cleanup of stopped containers and unused images")
		} else if !noInteraction {
			offerRuntimeCleanup(executor)
		}
	}
}

// extractBuildServices returns the sorted list of service names that have a
// dargstack.development.build label, meaning they would be auto-built.
func extractBuildServices(composeData []byte) []string {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return nil
	}

	var names []string
	for name, def := range svcMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}
		if extractDargstackBuildContext(svc) != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// extractGitServices returns the sorted list of service names that have a
// dargstack.development.git label, meaning their repos would be cloned.
func extractGitServices(composeData []byte) []string {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return nil
	}

	var names []string
	for name, def := range svcMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}
		if extractDargstackGitLabel(svc) != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// countComposeServices returns the number of services defined in the compose data.
func countComposeServices(composeData []byte) int {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return 0
	}
	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return 0
	}
	return len(svcMap)
}

// injectBuildContext adds a dargstack.development.build label to the specified
// service if one does not already exist. Returns the re-marshaled compose YAML.
func injectBuildContext(composeData []byte, serviceName, buildPath string) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("parse compose: %w", err)
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return composeData, nil
	}

	svcDef, ok := svcMap[serviceName].(map[string]interface{})
	if !ok {
		return composeData, nil
	}

	// If the service already has a .build label, don't override it.
	if extractDargstackBuildContext(svcDef) != "" {
		return composeData, nil
	}

	deploy, ok := svcDef["deploy"].(map[string]interface{})
	if !ok {
		deploy = map[string]interface{}{}
		svcDef["deploy"] = deploy
	}

	labels, ok := deploy["labels"]
	if !ok {
		labels = map[string]interface{}{}
		deploy["labels"] = labels
	}

	switch v := labels.(type) {
	case map[string]interface{}:
		v["dargstack.development.build"] = buildPath
	case []interface{}:
		v = append(v, "dargstack.development.build="+buildPath)
		deploy["labels"] = v
	}

	return yaml.Marshal(doc)
}

// cloneGitRepos clones git repositories for services with a
// dargstack.development.git label. It returns mutated compose data with
// .build labels injected where .git is set but .build is not.
func cloneGitRepos(stackDir string, composeData []byte) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("parse compose: %w", err)
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return composeData, nil
	}

	// Sibling directory of the stack directory
	parentDir := filepath.Dir(stackDir)

	for name, def := range svcMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}

		gitURL := extractDargstackGitLabel(svc)
		if gitURL == "" {
			continue
		}

		repoName := repoNameFromURL(gitURL)
		targetDir := filepath.Join(parentDir, repoName)

		if _, err := os.Stat(targetDir); err == nil {
			// Directory already exists — inject .build if missing, skip clone.
			composeData, err = injectBuildContext(composeData, name, targetDir)
			if err != nil {
				return nil, fmt.Errorf("service %q inject build context: %w", name, err)
			}
			continue
		}

		printInfo(fmt.Sprintf("Cloning %s for service %q", gitURL, name))
		cmd := exec.Command("git", "clone", "--depth", "1", gitURL, targetDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("clone %s for service %q: %s: %w", gitURL, name, strings.TrimSpace(string(out)), err)
		}

		// Run make init if a Makefile exists.
		makefile := filepath.Join(targetDir, "Makefile")
		if _, err := os.Stat(makefile); err == nil {
			printInfo(fmt.Sprintf("Initializing %s for service %q", repoName, name))
			initCmd := exec.Command("make", "init")
			initCmd.Dir = targetDir
			initOut, initErr := initCmd.CombinedOutput()
			if initErr != nil {
				printWarning(fmt.Sprintf("Init for service %q failed: %s", name, strings.TrimSpace(string(initOut))))
			}
		}

		composeData, err = injectBuildContext(composeData, name, targetDir)
		if err != nil {
			return nil, fmt.Errorf("service %q inject build context: %w", name, err)
		}
	}

	return composeData, nil
}

// uniqueSortedDomains merges two domain slices, removes duplicates, and returns
// the result sorted. This stabilises regeneration checks and avoids feeding
// duplicate entries to certificate generators.
func uniqueSortedDomains(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	for _, d := range a {
		if d != "" {
			seen[d] = true
		}
	}
	for _, d := range b {
		if d != "" {
			seen[d] = true
		}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

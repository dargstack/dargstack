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

	"charm.land/huh/v2/spinner"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"

	"github.com/dargstack/dargstack/v4/internal/audit"
	"github.com/dargstack/dargstack/v4/internal/compose"
	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/giturl"
	"github.com/dargstack/dargstack/v4/internal/logger"
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

	// Extract TLS domains from all services before profile filtering, so
	// certificates cover every domain regardless of which profile is active.
	allDomains := tls.FilterDomains(tls.ExtractDomains(composeData, cfg.Environment.Development.Domain), cfg.Environment.Development.Certificate.Include, cfg.Environment.Development.Certificate.Exclude)

	composeVars := applyEnvToProcess(isProduction())

	if dryRun {
		logger.L.Info("[dry-run] Would check for missing environment variable values")
	} else {
		if err := promptForEnvValues(isProduction()); err != nil {
			logger.L.Warn(fmt.Sprintf("Environment variable check: %v", err))
		}
		executor.SetComposeEnv(composeVars)
	}

	// Secret setup (before validation so generated secrets are present).
	secretIssues, err := deploySetupSecrets(composeData, dryRun)
	if err != nil {
		return err
	}

	// Filter for all subsequent operations so TLS, git cloning, repo
	// fetching, and auto-builds only operate on services that will be deployed.
	composeData, filterMsg, filterErr := applyProfileFilter(composeData)
	if filterErr != nil {
		return fmt.Errorf("filter compose by profile: %w", filterErr)
	}
	logger.L.Info(filterMsg)

	if !isProduction() {
		composeData, err = deployPrepareDevelopment(ctx, dockerClient, executor, composeData, allDomains, dryRun)
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

	// Validate resources after preparation so generated certs and cloned
	// repos are visible to the validator.
	if err := deployValidateResources(composeData, secretIssues, dryRun); err != nil {
		return err
	}

	if err := deployExecute(executor, composeData, env, dryRun); err != nil {
		return err
	}

	deployPostDeploy(ctx, dockerClient, executor, composeData, dryRun)

	return nil
}

// deploySetupSecrets filters composeData by profile and runs secret setup.
// Returns secret-related issues for later combination with validation results.
func deploySetupSecrets(composeData []byte, dryRun bool) ([]resource.Issue, error) {
	secretComposeData, _, filterErr := applyProfileFilter(composeData)
	if filterErr != nil {
		return nil, fmt.Errorf("filter compose by profile for secret setup: %w", filterErr)
	}

	if dryRun {
		templates, templateErr := secret.ExtractTemplates(secretComposeData)
		if templateErr == nil && len(templates) > 0 {
			logger.L.Info(fmt.Sprintf("[dry-run] Would set up %d secret(s):", len(templates)))
			for name, tmpl := range templates {
				switch {
				case tmpl.Type == secret.TypeThirdParty || tmpl.ThirdParty:
					msg := fmt.Sprintf("  %s: third-party (provide manually)", name)
					if tmpl.Hint != "" {
						msg += fmt.Sprintf(" — %s", tmpl.Hint)
					}
					logger.L.Info(msg)
				case tmpl.Type == secret.TypeTemplate || tmpl.Template != "":
					logger.L.Info(fmt.Sprintf("  %s: template", name))
				case tmpl.Type == secret.TypeWordlistWord:
					logger.L.Info(fmt.Sprintf("  %s: generated word", name))
				case tmpl.Type == secret.TypePrivateKey:
					logger.L.Info(fmt.Sprintf("  %s: generated private key", name))
				case tmpl.Type == secret.TypeInsecureDefault:
					logger.L.Info(fmt.Sprintf("  %s: insecure default value", name))
				case secret.IsAutoGeneratable(&tmpl):
					length := tmpl.Length
					if length <= 0 {
						length = 32
					}
					logger.L.Info(fmt.Sprintf("  %s: generated (%d chars)", name, length))
				default:
					logger.L.Info(fmt.Sprintf("  %s: interactive prompt required", name))
				}
			}
		} else {
			logger.L.Info("[dry-run] No secrets to set up")
		}
		return nil, nil
	}

	secretIssues, err, _ := secretSetupFlow(secretComposeData, isProduction(), true)
	if err != nil {
		return nil, fmt.Errorf("secret setup: %w", err)
	}
	return secretIssues, nil
}

// deployValidateResources runs resource validation and combines results with
// any pre-existing secret issues. Returns an error if validation fails.
func deployValidateResources(composeData []byte, secretIssues []resource.Issue, dryRun bool) error {
	if dryRun {
		logger.L.Info("[dry-run] Would validate stack resources")
		return nil
	}

	issues, err := resource.Validate(composeData, stackDir, isProduction())
	if err != nil {
		return wrapWithBugHint(err)
	}
	if printIssues(append(secretIssues, issues...)) {
		return hintErr(
			errors.New(ErrValidationFailed),
			"Fix the errors listed above, then run `dargstack deploy` again.",
		)
	}
	return nil
}

// deployPrepareDevelopment handles TLS certs, git clones, repo fetches,
// auto-builds, and volume cleanup for development deployments.
func deployPrepareDevelopment(ctx context.Context, dockerClient *docker.Client, executor *docker.Executor, composeData []byte, domains []string, dryRun bool) ([]byte, error) {
	// TLS certificates
	if dryRun {
		logger.L.Info(fmt.Sprintf("[dry-run] Would ensure TLS certificates for: %s", strings.Join(domains, ", ")))
	} else {
		certDir := cfg.CertificatesDir()
		if err := tls.EnsureCertificates(certDir, domains); err != nil {
			logger.L.Warn(fmt.Sprintf("TLS certificate setup failed: %v", err))
		}
	}

	// Clone git repos
	if dryRun {
		gitServices := extractGitServices(composeData)
		if len(gitServices) > 0 {
			logger.L.Info(fmt.Sprintf("[dry-run] Would clone and initialize repositories for: %s", strings.Join(gitServices, ", ")))
		}
	} else {
		var err error
		composeData, err = cloneGitRepos(stackDir, composeData)
		if err != nil {
			return nil, fmt.Errorf("clone git repos: %w", err)
		}
	}

	// Fetch build-context repos and warn if behind
	if !dryRun && !offline {
		behindRepos := fetchAndWarnBehind(composeData)
		printBehindWarning(behindRepos)
	}

	// Auto-build images
	buildServices := extractBuildServices(composeData)
	if dryRun {
		if len(buildServices) > 0 {
			logger.L.Info(fmt.Sprintf("[dry-run] Would auto-build images for: %s", strings.Join(buildServices, ", ")))
		} else {
			logger.L.Info("[dry-run] No services require auto-build")
		}
	} else {
		if err := autoBuildServices(executor, composeData); err != nil {
			return nil, fmt.Errorf("auto-build failed: %w", err)
		}
	}

	// Volume cleanup prompt
	if dryRun {
		logger.L.Info("[dry-run] Would prompt for volume cleanup (first-time deploy)")
	} else if !noInteraction {
		promptVolumes := *cfg.Runtime.Deploy.Volumes.Prompt
		if promptVolumes {
			running := isStackRunning(ctx, dockerClient, executor)
			if !running {
				ok, _ := prompt.Confirm("Remove all stack volumes for a clean start?", false)
				if ok {
					volumes, volErr := docker.VolumeList(executor, cfg.Metadata.Name)
					if volErr == nil && len(volumes) > 0 {
						if err := docker.VolumeRemove(executor, volumes); err != nil {
							logger.L.Warn(fmt.Sprintf("Failed to remove volumes: %v", err))
						} else {
							for _, v := range volumes {
								logger.L.Info(fmt.Sprintf("  Removed volume: %s", v))
							}
							logger.Success(fmt.Sprintf(MsgRemovedVolumes, len(volumes)))
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
			logger.L.Info(fmt.Sprintf("[dry-run] Would check image accessibility for: %s", strings.Join(images, ", ")))
		}
	} else if offline {
		if len(images) > 0 {
			logger.L.Info("Skipping image accessibility check (--offline)")
		}
	} else if len(images) > 0 {
		if verbose {
			logger.L.Info(fmt.Sprintf("Checking image accessibility for %d image(s) in parallel: %s", len(images), strings.Join(images, ", ")))
		}

		var unreachable map[string]error
		checkImages := func() {
			unreachable = docker.CheckImagesAccessible(executor, images)
		}

		if verbose {
			checkImages()
		} else {
			if err := spinner.New().Title("Checking image accessibility").Action(checkImages).Run(); err != nil {
				return nil, err
			}
		}

		if len(unreachable) > 0 {
			unreachableImages := make([]string, 0, len(unreachable))
			for img := range unreachable {
				unreachableImages = append(unreachableImages, img)
			}
			sort.Strings(unreachableImages)
			for _, img := range unreachableImages {
				logger.L.Error(fmt.Sprintf("image not accessible: %s: %v", img, unreachable[img]))
			}
			return nil, fmt.Errorf("one or more images are not accessible — fix registry credentials or image references before deploying")
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
		if cfg.Environment.Production.Domain == "app.localhost" {
			prefix := ""
			if dryRun {
				prefix = "[dry-run] "
			}
			logger.L.Warn(prefix + "STACK_DOMAIN is still set to default \"app.localhost\" — set domain in dargstack.yaml for production")
		}
		tag := "unknown"
		if !dryRun {
			resolvedTag, tagErr := resolveDeployTag()
			if tagErr != nil {
				logger.L.Warn(fmt.Sprintf("Deploy tag resolution failed: %v", tagErr))
			} else {
				tag = resolvedTag
			}
		}
		if dryRun {
			logger.L.Info(fmt.Sprintf("[dry-run] Would deploy production stack %q (tag: %s)", cfg.Metadata.Name, tag))
		} else {
			logger.L.Info(fmt.Sprintf("Deploying production stack %q (tag: %s)", cfg.Metadata.Name, tag))
		}
	} else {
		if dryRun {
			logger.L.Info(fmt.Sprintf("[dry-run] Would deploy development stack %q", cfg.Metadata.Name))
		} else {
			logger.L.Info(fmt.Sprintf("Deploying development stack %q", cfg.Metadata.Name))
		}
	}

	// Save audit trail
	if dryRun {
		logger.L.Info("[dry-run] Would save deployment snapshot to audit log")
	} else {
		auditDir := audit.AuditLogDir(stackDir)
		if snapPath, saveErr := audit.SaveDeployment(auditDir, env, composeData); saveErr == nil {
			logger.L.Debug(fmt.Sprintf("Deployment snapshot: %s", snapPath))
		} else {
			logger.L.Warn(fmt.Sprintf("Failed to save audit snapshot: %v", saveErr))
		}
	}

	// Execute deployment
	if dryRun {
		logger.L.Info("[dry-run] Would execute `docker stack deploy`")
		logger.L.Info("[dry-run] Final compose output:")
		fmt.Println()
		fmt.Print(string(composeData))
	} else {
		if err := docker.StackDeploy(executor, cfg.Metadata.Name, composeData); err != nil {
			return wrapWithBugHint(err)
		}
	}

	return nil
}

// deployPostDeploy prints service count and offers runtime cleanup.
func deployPostDeploy(ctx context.Context, dockerClient *docker.Client, executor *docker.Executor, composeData []byte, dryRun bool) {
	if dryRun {
		svcCount := countComposeServices(composeData)
		logger.L.Info(fmt.Sprintf("[dry-run] Would have %d service(s) running", svcCount))
	} else {
		if count, err := countStackServices(ctx, dockerClient, executor); err == nil {
			logger.Success(fmt.Sprintf("Stack %q deployed with %d service(s)", cfg.Metadata.Name, count))
		}
	}

	if isProduction() {
		if dryRun {
			logger.L.Info("[dry-run] Would offer runtime cleanup of stopped containers and unused images")
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
// dargstack.development.git.ssh or dargstack.development.git.https label, meaning their repos would be cloned.
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
		if giturl.ExtractFromService(svc, name).IsSet() {
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
// dargstack.development.git.ssh or dargstack.development.git.https label. It returns mutated compose data with
// .build labels injected where .git.ssh/.git.https is set but .build is not.
func cloneGitRepos(stackDir string, composeData []byte) ([]byte, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("parse compose: %w", err)
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return composeData, nil
	}

	hasGitRepos := false
	for _, def := range svcMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}
		if giturl.ExtractFromService(svc, "").IsSet() {
			hasGitRepos = true
			break
		}
	}
	if !hasGitRepos {
		return composeData, nil
	}

	var result []byte
	err := spinner.New().
		Title("Cloning repositories").
		ActionWithErr(func(ctx context.Context) error {
			result = composeData
			parentDir := filepath.Dir(stackDir)

			for name, def := range svcMap {
				svc, ok := def.(map[string]interface{})
				if !ok {
					continue
				}

				gitURL := giturl.ExtractFromService(svc, name)
				if !gitURL.IsSet() {
					continue
				}

				repoName := giturl.RepoNameFromURL(gitURL.Primary())
				targetDir := filepath.Join(parentDir, repoName)

				if _, statErr := os.Stat(targetDir); statErr == nil {
					var injectErr error
					result, injectErr = injectBuildContext(result, name, targetDir)
					if injectErr != nil {
						return fmt.Errorf("service %q inject build context: %w", name, injectErr)
					}
					continue
				}

				cloneErr := giturl.CloneWithFallback(gitURL, targetDir)
				if cloneErr != nil {
					return fmt.Errorf("clone repository for service %q: %w", name, cloneErr)
				}

				makefile := filepath.Join(targetDir, "Makefile")
				if _, statErr := os.Stat(makefile); statErr == nil {
					logger.L.Warn(fmt.Sprintf("service %q: running `make init` on cloned repositories is deprecated and will be removed in a future version", name))
					logger.L.Info(fmt.Sprintf("Initializing %s for service %q", repoName, name))
					initCmd := exec.Command("make", "init")
					initCmd.Dir = targetDir
					initOut, initErr := initCmd.CombinedOutput()
					if initErr != nil {
						logger.L.Warn(fmt.Sprintf("Init for service %q failed: %s", name, strings.TrimSpace(string(initOut))))
					}
				}

				var injectErr error
				result, injectErr = injectBuildContext(result, name, targetDir)
				if injectErr != nil {
					return fmt.Errorf("service %q inject build context: %w", name, injectErr)
				}
			}
			return nil
		}).
		Run()
	if err != nil {
		return nil, err
	}
	return result, nil
}

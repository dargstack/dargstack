package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/swarm"
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

var (
	production   bool
	profiles     []string
	services     []string
	deployTag    string
	dryRun       bool
	listSecrets  bool
	listProfiles bool
	secretsOnly  bool
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the stack",
	Long: `Deploy services to a Docker Swarm stack.

By default, deploys to the development environment. This includes:
- Auto-building images for services with dargstack.development.build labels (unless behavior.build.skip is set)
- Generating TLS certificates for local development
- Setting up secrets interactively or with defaults
- Validating all stack resources

use --production to deploy to production, which:
- Requires all environment variables and secrets to be set
- Blocks deployment if default insecure secrets are present
- Pre-pulls images before deployment
- Includes production-only services

Use --profiles to activate specific compose profiles.
Use --services to deploy only selected services.
Use --dry-run to preview all steps without deploying.
Use --list-profiles to print discovered profiles and exit.
Use --list-secrets to print resolved secrets and exit.
Use --secrets-only to run secret setup only without deploying.
Use --tag (production only) to deploy a specific git tag.`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().BoolVarP(&production, "production", "p", false, "deploy in production mode")
	deployCmd.Flags().StringSliceVar(&profiles, "profiles", nil, "activate one or more compose profiles; unlabeled services are included unless a 'default' profile is defined")
	deployCmd.Flags().StringSliceVar(&services, "services", nil, "deploy only these services (comma-separated)")
	deployCmd.Flags().StringVar(&deployTag, "tag", "", "deploy a specific git tag (production only)")
	deployCmd.Flags().BoolVar(&dryRun, "dry-run", false, "trace all steps without deploying")
	deployCmd.Flags().BoolVar(&listProfiles, "list-profiles", false, "list discovered deploy profiles and exit")
	deployCmd.Flags().BoolVar(&listSecrets, "list-secrets", false, "list resolved secrets and exit")
	deployCmd.Flags().BoolVar(&secretsOnly, "secrets-only", false, "run secret setup only without deploying")
}

func runDeploy(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	env := "development"
	if production {
		env = "production"
	}

	if dryRun {
		printInfo(fmt.Sprintf("[dry-run] Tracing %s deployment for stack %q", env, cfg.Name))
	}

	if listProfiles || listSecrets {
		return runDeployListMode()
	}

	if secretsOnly {
		return runSecretsOnly()
	}

	// 1. Docker prerequisite check — create executor first so sudo is pre-warmed
	// before any Docker socket access.
	if !dryRun {
		executor, err := docker.NewExecutor(cfg.Sudo)
		if err != nil {
			return wrapWithBugHint(err)
		}

		// When sudo is needed the Docker SDK cannot reach the socket directly,
		// so perform all pre-flight checks through the CLI executor.
		if executor.NeedsSudo() {
			if err := executor.Ping(); err != nil {
				return hintErr(
					fmt.Errorf("docker is not running: %w", err),
					"Start Docker Desktop or the docker daemon, then try again.",
				)
			}

			swarmActive, err := executor.SwarmActive()
			if err != nil {
				return wrapWithBugHint(err)
			}
			if !swarmActive {
				if err := ensureSwarm(executor); err != nil {
					return err
				}
			}

			return runDeployWithExecutor(ctx, cmd, nil, executor, env)
		}

		// No sudo required — use the SDK for richer checks.
		dockerClient, err := docker.NewClient()
		if err != nil {
			return wrapWithBugHint(err)
		}
		defer func() { _ = dockerClient.Close() }()

		if err := dockerClient.Ping(ctx); err != nil {
			return hintErr(
				fmt.Errorf("docker is not running: %w", err),
				"Start Docker Desktop or the docker daemon, then try again.",
			)
		}

		// 2. Swarm check
		swarmState, err := dockerClient.SwarmStatus(ctx)
		if err != nil {
			return wrapWithBugHint(err)
		}

		if swarmState != swarm.LocalNodeStateActive {
			if err := ensureSwarm(executor); err != nil {
				return err
			}
		}

		return runDeployWithExecutor(ctx, cmd, dockerClient, executor, env)
	}

	// Dry-run path — no Docker interaction needed
	return runDeployDryRun(env)
}

func runDeployListMode() error {
	var composeData []byte
	var err error
	if production {
		composeData, err = buildProductionCompose()
	} else {
		composeData, err = buildDevelopmentCompose()
	}
	if err != nil {
		return wrapWithBugHint(err)
	}

	if listProfiles {
		profiles, profErr := compose.DiscoverProfiles(composeData)
		if profErr != nil {
			return profErr
		}
		sort.Strings(profiles)
		if len(profiles) == 0 {
			printInfo("No profiles found")
		} else {
			printInfo("Discovered profiles:")
			for _, p := range profiles {
				fmt.Printf("- %s\n", p)
			}
		}
	}

	if listSecrets {
		paths := secret.ExtractSecretPaths(composeData)
		if len(paths) == 0 {
			printInfo("No secrets found")
		} else {
			values := secret.ReadSecretValues(paths)
			names := make([]string, 0, len(paths))
			for name := range paths {
				names = append(names, name)
			}
			sort.Strings(names)

			jsonOutput := noInteraction || strings.EqualFold(outputFormat, "json")

			switch {
			case jsonOutput:
				entries := make([]map[string]string, 0, len(names))
				for _, name := range names {
					entries = append(entries, map[string]string{
						"name":  name,
						"file":  paths[name],
						"value": values[name],
					})
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(entries); err != nil {
					return err
				}
			case hasClipboardSupport():
				for i, name := range names {
					for {
						title := fmt.Sprintf("Secret %d/%d: %s", i+1, len(names), name)
						choice, choiceErr := prompt.Select(title, []string{
							"Copy key to clipboard",
							"Copy value to clipboard",
							"Next",
							"Done",
						})
						if choiceErr != nil {
							return choiceErr
						}

						switch choice {
						case "Copy key to clipboard":
							if copyErr := copyToClipboard(name); copyErr != nil {
								printWarning(fmt.Sprintf("Clipboard copy failed: %v", copyErr))
							} else {
								printSuccess(fmt.Sprintf("Copied key %q", name))
							}
						case "Copy value to clipboard":
							if copyErr := copyToClipboard(values[name]); copyErr != nil {
								printWarning(fmt.Sprintf("Clipboard copy failed: %v", copyErr))
							} else {
								printSuccess(fmt.Sprintf("Copied value for %q", name))
							}
						case "Done":
							return nil
						default: // Next
							goto nextSecret
						}
					}
				nextSecret:
				}
			default:
				printWarning("No clipboard tool found. Falling back to table output.")
				nameWidth := len("NAME")
				for _, name := range names {
					if len(name) > nameWidth {
						nameWidth = len(name)
					}
				}

				fmt.Printf("%-*s  %s\n", nameWidth, "NAME", "VALUE")
				fmt.Printf("%-*s  %s\n", nameWidth, strings.Repeat("-", nameWidth), strings.Repeat("-", 5))
				for _, name := range names {
					fmt.Printf("%-*s  %s\n", nameWidth, name, values[name])
				}
			}
		}
	}

	return nil
}

func runSecretsOnly() error {
	var composeData []byte
	var err error
	if production {
		composeData, err = buildProductionCompose()
	} else {
		composeData, err = buildDevelopmentCompose()
	}
	if err != nil {
		return wrapWithBugHint(err)
	}

	composeData, err = applyProfileFilter(composeData)
	if err != nil {
		return fmt.Errorf("filter compose by profile: %w", err)
	}

	if err := secretSetupFlow(composeData, production); err != nil {
		return err
	}

	printSuccess("Secret setup complete. Run `dargstack deploy` to deploy.")
	return nil
}

func runDeployDryRun(env string) error {
	printInfo("[dry-run] Step 1: Collecting compose sources...")

	var composeData []byte
	var err error
	if production {
		composeData, err = buildProductionCompose()
	} else {
		composeData, err = buildDevelopmentCompose()
	}
	if err != nil {
		return wrapWithBugHint(err)
	}

	applyEnvToProcess(production)
	printInfo(fmt.Sprintf("[dry-run] Step 2: Compose merged (%d bytes)", len(composeData)))

	// Show profile filtering
	switch {
	case len(profiles) > 0:
		printInfo(fmt.Sprintf("[dry-run] Step 3: Filtering by profiles %v", profiles))
		composeData, err = compose.FilterByProfile(composeData, profiles)
		if err != nil {
			return err
		}
	case len(services) > 0:
		printInfo(fmt.Sprintf("[dry-run] Step 3: Filtering to services: %s", strings.Join(services, ", ")))
		composeData, err = compose.FilterServices(composeData, services)
		if err != nil {
			return err
		}
	default:
		if production {
			printInfo("[dry-run] Step 3: Production mode — deploying all services (no default profile filter)")
		} else {
			defaultProfileExists := composeHasProfile(composeData, "default")
			printInfo("[dry-run] Step 3: Applying default profile semantics")
			composeData, err = compose.FilterByProfile(composeData, nil)
			if err != nil {
				return err
			}
			if defaultProfileExists {
				printInfo("[dry-run] Default profile detected: deploying only services in profile \"default\"")
			} else {
				printInfo("[dry-run] No default profile detected: deploying all services")
			}
		}
	}

	// Show domain extraction
	if !production {
		domains := uniqueSortedDomains(tls.ExtractDomains(composeData, cfg.Production.Domain), cfg.Development.Domains)
		printInfo(fmt.Sprintf("[dry-run] Step 4: TLS domains discovered: %s", strings.Join(domains, ", ")))
	}

	// Show secret template summary
	templates, templateErr := secret.ExtractTemplates(composeData)
	if templateErr == nil && len(templates) > 0 {
		printInfo(fmt.Sprintf("[dry-run] Step 5: Secret templates found: %d", len(templates)))
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
			case tmpl.Type == secret.TypeWord:
				printInfo(fmt.Sprintf("  %s: generated word", name))
			case tmpl.Type == secret.TypePrivateKey:
				printInfo(fmt.Sprintf("  %s: generated private key", name))
			case tmpl.Type == secret.TypeInsecureValue:
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
		printInfo("[dry-run] Step 5: No secret templates found")
	}

	// In production output, strip dargstack.development.* labels.
	if production {
		composeData, err = compose.StripProductionDevelopmentLabels(composeData)
		if err != nil {
			return wrapWithBugHint(err)
		}
	}

	printInfo("[dry-run] Step 6: Final compose output:")
	fmt.Println()
	fmt.Print(string(composeData))
	return nil
}

func runDeployWithExecutor(ctx context.Context, _ *cobra.Command, dockerClient *docker.Client, executor *docker.Executor, env string) error {
	// 3. Build compose data (services-only architecture)
	var composeData []byte
	var err error
	if production {
		composeData, err = buildProductionCompose()
	} else {
		composeData, err = buildDevelopmentCompose()
	}
	if err != nil {
		return wrapWithBugHint(err)
	}

	// 4. Environment variable prompt
	if err := promptForEnvValues(production); err != nil {
		printWarning(fmt.Sprintf("Environment variable check: %v", err))
	}
	composeVars := applyEnvToProcess(production)
	executor.SetComposeEnv(composeVars)

	// 5. Secret setup — filter by profile first so only secrets used by active
	// services are processed.
	secretComposeData, filterErr := applyProfileFilter(composeData)
	if filterErr != nil {
		return fmt.Errorf("filter compose by profile for secret setup: %w", filterErr)
	}
	if err := secretSetupFlow(secretComposeData, production); err != nil {
		return fmt.Errorf("secret setup: %w", err)
	}

	// Warn if any secrets still hold the UNSET THIRD PARTY SECRET placeholder.
	if placeholders := secret.PlaceholderSecrets(secretComposeData, stackDir); len(placeholders) > 0 {
		printWarning(fmt.Sprintf("Third party secrets hold placeholder value: %s", strings.Join(placeholders, ", ")))
		printInfo("Replace those secrets with real values before using this stack with third party APIs.")
	}

	// 6. Validate resources
	issues, err := resource.Validate(secretComposeData, stackDir, production)
	if err != nil {
		return wrapWithBugHint(err)
	}

	hasErrors := false
	for _, iss := range issues {
		if iss.Severity == "error" {
			printError(iss.String())
			hasErrors = true
		} else {
			printWarning(iss.String())
		}
	}

	if hasErrors {
		missing := resource.MissingSecrets(issues)
		if len(missing) > 0 {
			printInfo("Tip: Define missing secrets in x-dargstack.secrets with typed secret metadata to auto-generate them during deploy.")
		}
		return hintErr(
			fmt.Errorf("validation failed"),
			"Fix the errors listed above, then run `dargstack deploy` again.",
		)
	}

	// 7. TLS certificates (development only) with domain-aware regeneration
	if !production {
		domains := uniqueSortedDomains(tls.ExtractDomains(composeData, cfg.Production.Domain), cfg.Development.Domains)
		certDir := config.CertificatesDir(stackDir)
		if err := tls.EnsureCertificates(certDir, domains); err != nil {
			printWarning(fmt.Sprintf("TLS certificate setup failed: %v", err))
		}
	}

	// 8. Auto-build images (development only)
	if !production {
		if err := autoBuildServices(executor, composeData); err != nil {
			printWarning(fmt.Sprintf("Auto-build failed: %v", err))
		}
	}

	// 9. Pre-pull images and warn about start-first (production only)
	if production {
		prePullAndWarn(executor, composeData)
	}

	// 10. Volume cleanup prompt (development, not already running)
	if !production && !noInteraction {
		// Check behavior.prompt.volume.remove (defaults to true)
		promptVolumes := true
		if cfg.Behavior.Prompt != nil && cfg.Behavior.Prompt.Volume != nil {
			promptVolumes = cfg.Behavior.Prompt.Volume.Remove
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
							printSuccess(fmt.Sprintf("Removed %d volume(s)", len(volumes)))
						}
					}
				}
			}
		}
	}

	// 11. Filter for profile/services
	switch {
	case len(profiles) > 0:
		composeData, err = compose.FilterByProfile(composeData, profiles)
		if err != nil {
			return fmt.Errorf("filter profiles %v: %w", profiles, err)
		}
		printInfo(fmt.Sprintf("Deploying with profiles %v active", profiles))
	case len(services) > 0:
		composeData, err = compose.FilterServices(composeData, services)
		if err != nil {
			return fmt.Errorf("filter services: %w", err)
		}
		printInfo(fmt.Sprintf("Deploying services: %s", strings.Join(services, ", ")))
	default:
		if production {
			// Production deploys all services by default; --profiles or --services
			// can still scope the deployment explicitly.
			printInfo("Production deployment: deploying all services")
		} else {
			defaultProfileExists := composeHasProfile(composeData, "default")
			composeData, err = compose.FilterByProfile(composeData, nil)
			if err != nil {
				return fmt.Errorf("apply default profile semantics: %w", err)
			}
			if defaultProfileExists {
				printInfo("Default profile detected: deploying only services in profile \"default\" (use --profiles unlabeled to include unlabeled services)")
			} else {
				printInfo("No default profile detected: deploying all services")
			}
		}
	}

	// 12. For production, strip dargstack.development.* before deploying.
	if production {
		composeData, err = compose.StripProductionDevelopmentLabels(composeData)
		if err != nil {
			return wrapWithBugHint(err)
		}
	}

	// 13. Deploy
	if production {
		if cfg.Production.Domain == "app.localhost" {
			printWarning("STACK_DOMAIN is still set to default \"app.localhost\" — set domain in dargstack.yaml for production")
		}
		tag, tagErr := resolveDeployTag()
		if tagErr != nil {
			printWarning(fmt.Sprintf("Deploy tag resolution failed: %v", tagErr))
			tag = "unknown"
		}
		printInfo(fmt.Sprintf("Deploying production stack %q (tag: %s)", cfg.Name, tag))
	} else {
		printInfo(fmt.Sprintf("Deploying development stack %q", cfg.Name))
	}

	// 14. Save audit trail (before deployment)
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

	// 15. Deploy
	if err := docker.StackDeploy(executor, cfg.Name, composeData); err != nil {
		return wrapWithBugHint(err)
	}

	// 16. Post-deploy status
	if count, err := countStackServices(ctx, dockerClient, executor); err == nil {
		printSuccess(fmt.Sprintf("Stack %q deployed with %d service(s)", cfg.Name, count))
	}

	// 17. Offer to clean up stopped containers and unused images (production)
	if production && !noInteraction {
		offerRuntimeCleanup(executor)
	}

	return nil
}

func ensureSwarm(executor *docker.Executor) error {
	if noInteraction {
		return hintErr(
			fmt.Errorf("docker swarm is not initialized"),
			"Run `docker swarm init` to initialize, or remove --no-interaction to let dargstack do it for you.",
		)
	}
	ok, promptErr := prompt.Confirm("Docker Swarm is not initialized. Initialize now?", true)
	if promptErr != nil || !ok {
		return hintErr(
			fmt.Errorf("docker swarm is required for deployment"),
			"Run `docker swarm init` manually to initialize.",
		)
	}
	if err := initSwarmWithAddrSelection(executor); err != nil {
		return wrapWithBugHint(err)
	}
	printSuccess("Swarm initialized")
	return nil
}

// isStackRunning checks if the stack has running services, using the SDK client
// when available or falling back to the executor.
func isStackRunning(ctx context.Context, client *docker.Client, executor *docker.Executor) bool {
	if client != nil {
		running, err := client.IsStackRunning(ctx, cfg.Name)
		return err == nil && running
	}
	out, err := executor.Run("stack", "services", "--quiet", cfg.Name)
	return err == nil && strings.TrimSpace(out) != ""
}

// countStackServices returns the number of services in the deployed stack.
func countStackServices(ctx context.Context, client *docker.Client, executor *docker.Executor) (int, error) {
	if client != nil {
		svcList, err := client.ListStackServices(ctx, cfg.Name)
		if err != nil {
			return 0, err
		}
		return len(svcList), nil
	}
	out, err := executor.Run("stack", "services", "--quiet", cfg.Name)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0, nil
	}
	return len(lines), nil
}

func initSwarmWithAddrSelection(executor *docker.Executor) error {
	err := docker.SwarmInit(executor)
	if err == nil {
		return nil
	}

	addrs, listErr := docker.ListAdvertiseAddrs()
	if listErr != nil || len(addrs) == 0 {
		return err
	}

	if noInteraction {
		return docker.SwarmInitWithAddr(executor, addrs[0])
	}

	addr, promptErr := prompt.Select("Select advertise address for Docker Swarm:", addrs)
	if promptErr != nil {
		return err
	}
	return docker.SwarmInitWithAddr(executor, addr)
}

func buildDevelopmentCompose() ([]byte, error) {
	var paths []string

	// Service files (services-only architecture)
	svcFiles, err := config.CollectServiceFiles(config.DevDir(stackDir))
	if err != nil {
		return nil, err
	}
	paths = append(paths, svcFiles...)

	if len(paths) == 0 {
		return nil, hintErr(
			fmt.Errorf("no compose sources found"),
			fmt.Sprintf("Create service directories in %s, each containing a compose.yaml.", config.DevDir(stackDir)),
		)
	}

	var data []byte
	if len(paths) == 1 {
		data, err = compose.LoadSingle(paths[0])
	} else {
		data, err = compose.MergeFiles(paths...)
	}
	if err != nil {
		return nil, err
	}

	data, err = secret.RewriteSecretFilePaths(data, config.SecretsDir(stackDir))
	if err != nil {
		return nil, fmt.Errorf("rewrite secret file paths: %w", err)
	}

	return data, nil
}

func buildProductionCompose() ([]byte, error) {
	var paths []string

	// Dev sources (base layer)
	devSvcFiles, err := config.CollectServiceFiles(config.DevDir(stackDir))
	if err != nil {
		return nil, err
	}
	paths = append(paths, devSvcFiles...)

	// Production overlays
	prodSvcFiles, err := config.CollectServiceFiles(config.ProdDir(stackDir))
	if err != nil {
		return nil, err
	}
	paths = append(paths, prodSvcFiles...)

	if len(paths) == 0 {
		return nil, fmt.Errorf("no compose sources found")
	}

	merged, err := compose.MergeFiles(paths...)
	if err != nil {
		return nil, err
	}

	// Strip dev-only markers in production
	merged = compose.StripDevOnlyMarkers(merged)

	// Remap bind mounts from development paths to mirrored production paths
	// when production files/directories exist.
	if remapped, remapErr := compose.RewriteProductionBindMounts(
		merged,
		config.DevDir(stackDir),
		config.ProdDir(stackDir),
	); remapErr == nil {
		merged = remapped
	} else {
		printWarning(fmt.Sprintf("Failed to rewrite production bind mounts: %v", remapErr))
	}

	// Merge env files for production
	devEnv := config.DevEnvFile(stackDir)
	prodEnv := config.ProdEnvFile(stackDir)
	mergedEnv, mergeErr := compose.MergeEnvFiles(devEnv, prodEnv)
	if mergeErr != nil {
		printWarning(fmt.Sprintf("Failed to merge env files: %v", mergeErr))
	} else if len(mergedEnv) > 0 {
		envPath := filepath.Join(config.ArtifactsDir(stackDir), ".env.merged")
		if mkErr := os.MkdirAll(filepath.Dir(envPath), 0o755); mkErr == nil {
			_ = os.WriteFile(envPath, mergedEnv, 0o644)
		}
	}

	merged, err = secret.RewriteSecretFilePaths(merged, config.SecretsDir(stackDir))
	if err != nil {
		return nil, fmt.Errorf("rewrite secret file paths: %w", err)
	}

	return merged, nil
}

// applyEnvToProcess loads the resolved .env values and STACK_DOMAIN into the
// current process environment so that `docker stack deploy -c -` can
// interpolate them. The process environment takes precedence — existing values
// are not overwritten.
// It returns the map of vars that were applied so the caller can forward them
// explicitly to sudo subprocesses via Executor.SetComposeEnv.
func applyEnvToProcess(prod bool) map[string]string {
	env, err := compose.LoadEnvFile(config.DevEnvFile(stackDir))
	if err != nil {
		env = map[string]string{}
	}
	if prod {
		if prodEnv, pErr := compose.LoadEnvFile(config.ProdEnvFile(stackDir)); pErr == nil {
			for k, v := range prodEnv {
				env[k] = v
			}
		}
	}
	applied := make(map[string]string, len(env)+1)
	for k, v := range env {
		if v != "" && os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
		if v != "" {
			applied[k] = v
		}
	}
	if os.Getenv("STACK_DOMAIN") == "" {
		_ = os.Setenv("STACK_DOMAIN", cfg.Production.Domain)
	}
	applied["STACK_DOMAIN"] = cfg.Production.Domain
	return applied
}

func promptForEnvValues(prod bool) error {
	envPath := config.DevEnvFile(stackDir)
	env, err := compose.LoadEnvFile(envPath)
	if err != nil {
		return nil // no env file is fine
	}

	if prod {
		// For production, also load prod env
		prodEnv, prodErr := compose.LoadEnvFile(config.ProdEnvFile(stackDir))
		if prodErr == nil {
			for k, v := range prodEnv {
				env[k] = v
			}
		}
	}

	missing := compose.FindMissingEnvValues(env)
	if len(missing) == 0 {
		return nil
	}

	if prod {
		return hintErr(
			fmt.Errorf("production deployment requires all environment variables to be set — missing: %s", strings.Join(missing, ", ")),
			fmt.Sprintf("Add the missing values to %s.", config.ProdEnvFile(stackDir)),
		)
	}

	if noInteraction {
		printWarning(fmt.Sprintf("Missing environment variable values: %s", strings.Join(missing, ", ")))
		return nil
	}

	printInfo(fmt.Sprintf("Found %d environment variable(s) without values", len(missing)))
	ok, promptErr := prompt.Confirm("Fill in missing environment variable values now?", true)
	if promptErr != nil || !ok {
		return nil
	}

	for _, key := range missing {
		val, inputErr := prompt.Input(fmt.Sprintf("Value for %s:", key), "")
		if inputErr != nil {
			return inputErr
		}
		if val != "" {
			env[key] = val
		}
	}

	return compose.WriteEnvFile(envPath, env)
}

func secretSetupFlow(composeData []byte, prod bool) error {
	templates, err := secret.ExtractTemplates(composeData)
	if err != nil || len(templates) == 0 {
		return nil
	}

	if prod {
		// In production, third-party secrets that still hold the placeholder are an error.
		secretPaths := secret.ExtractSecretPaths(composeData)
		if len(secretPaths) > 0 {
			var placeholderThirdParty []string
			for name, tmpl := range templates {
				if !tmpl.ThirdParty && tmpl.Type != secret.TypeThirdParty {
					continue
				}
				if path, ok := secretPaths[name]; ok {
					data, err := os.ReadFile(path)
					if err != nil || secret.IsPlaceholderValue(strings.TrimSpace(string(data))) {
						placeholderThirdParty = append(placeholderThirdParty, name)
					}
				}
			}
			if len(placeholderThirdParty) > 0 {
				sort.Strings(placeholderThirdParty)
				return hintErr(
					fmt.Errorf("production deployment blocked: third-party secrets still hold placeholder values: %s", strings.Join(placeholderThirdParty, ", ")),
					"Replace those secret files with real values before deploying to production.",
				)
			}
		}
		return nil
	}

	// Extract file paths from compose for reading/writing secret values
	secretPaths := secret.ExtractSecretPaths(composeData)
	if len(secretPaths) == 0 {
		return nil
	}

	// Read existing values
	values := secret.ReadSecretValues(secretPaths)
	preResolved := make(map[string]string, len(values))
	for k, v := range values {
		preResolved[k] = v
	}

	missing := make([]string, 0, len(secretPaths))
	thirdParty := make(map[string]bool)
	for name, tmpl := range templates {
		if tmpl.ThirdParty || tmpl.Type == secret.TypeThirdParty {
			thirdParty[name] = true
		}
	}
	for name := range secretPaths {
		// Only prompt for secrets whose file does not exist at all.
		// Files that exist (even with placeholder content) are not reprompted —
		// third-party placeholders warn on deploy; other placeholders warn below.
		if secret.SecretFileExists(secretPaths, name) {
			continue
		}
		if !thirdParty[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)

	if len(missing) == 0 {
		if len(thirdParty) > 0 {
			var noFile []string
			// Write UNSET placeholder for any third-party secret with no file at all.
			for name := range thirdParty {
				if !secret.SecretFileExists(secretPaths, name) {
					noFile = append(noFile, name)
				}
			}
			if len(noFile) > 0 {
				sort.Strings(noFile)
				printInfo("Skipping local generation for third-party secrets")
				for _, name := range noFile {
					if tmpl, ok := templates[name]; ok && tmpl.Hint != "" {
						printInfo(fmt.Sprintf("  %s: expected value — %s", name, tmpl.Hint))
					}
					values[name] = secret.ThirdPartyPlaceholder
				}
				printWarning(fmt.Sprintf("Third-party secrets still unset: %s", strings.Join(noFile, ", ")))
				printInfo("Replace those secret files with real values before deploying to production.")
				_ = secret.WriteSecrets(secretPaths, values)
			}
		}
		return nil
	}

	if noInteraction {
		printWarning(fmt.Sprintf("Unset secrets: %s — run interactively to set them", strings.Join(missing, ", ")))
		printInfo("Tip: Add x-dargstack.secrets entries with typed secret metadata to auto-generate missing secrets.")
		return nil
	}

	manualInput := make(map[string]bool)
	skipAllAuto := false

	for _, name := range missing {
		if values[name] != "" {
			continue
		}

		tmpl, hasTemplate := templates[name]
		autoResolvable := hasTemplate && secret.IsAutoGeneratable(&tmpl)

		if autoResolvable {
			if skipAllAuto {
				continue
			}

			choice, choiceErr := prompt.Select(
				fmt.Sprintf("Secret %s", name),
				[]string{
					"Auto-generate this and remaining auto-generatable secrets (skip all)",
					"Auto-generate this secret (skip)",
					"Enter value",
				},
			)
			if choiceErr != nil {
				return choiceErr
			}

			if choice == "Auto-generate this and remaining auto-generatable secrets (skip all)" {
				skipAllAuto = true
				continue
			}
			if choice == "Auto-generate this secret (skip)" {
				continue
			}
		}

		promptText := fmt.Sprintf("Value for secret %s:", name)
		val, inputErr := prompt.Password(promptText)
		if inputErr != nil {
			return inputErr
		}
		if val != "" {
			values[name] = val
			manualInput[name] = true
		}
	}

	// Auto-generate any values that can be derived from x-dargstack.secrets.
	values, err = secret.Resolve(templates, values)
	if err != nil {
		return err
	}

	autoResolvedCount := 0
	for name, tmpl := range templates {
		if preResolved[name] != "" || manualInput[name] {
			continue
		}
		if values[name] == "" {
			continue
		}
		if secret.IsAutoGeneratable(&tmpl) {
			autoResolvedCount++
		}
	}
	if autoResolvedCount > 0 {
		printInfo(fmt.Sprintf("Auto-generated %d secret(s) from x-dargstack.secrets", autoResolvedCount))
		printInfo("Review generated values with `dargstack deploy --list-secrets`.")
	}

	// Write UNSET THIRD PARTY SECRET placeholder for third-party secrets that still have no file.
	for name := range thirdParty {
		if !secret.SecretFileExists(secretPaths, name) {
			values[name] = secret.ThirdPartyPlaceholder
		}
	}

	stillMissing := make([]string, 0, len(secretPaths))
	for name := range secretPaths {
		if values[name] == "" && !thirdParty[name] {
			stillMissing = append(stillMissing, name)
		}
	}
	sort.Strings(stillMissing)
	if len(stillMissing) > 0 {
		printWarning(fmt.Sprintf("Unset secrets remain: %s", strings.Join(stillMissing, ", ")))
		printInfo("Tip: Add x-dargstack.secrets entries with typed secret metadata to auto-generate missing secrets.")
	}

	return secret.WriteSecrets(secretPaths, values)
}

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

// prePullAndWarn pre-pulls images for production services and warns if start-first is missing.
func prePullAndWarn(executor *docker.Executor, composeData []byte) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return
	}

	for name, def := range svcMap {
		svc, ok := def.(map[string]interface{})
		if !ok {
			continue
		}

		if !hasStartFirst(svc) {
			printWarning(fmt.Sprintf("Service %q lacks deploy.update_config.order: start-first — updates may cause downtime", name))
		}

		if img, ok := svc["image"].(string); ok {
			printInfo(fmt.Sprintf("Pre-pulling %s", img))
			_, _ = executor.Run("pull", img)
		}
	}
}

func hasStartFirst(svc map[string]interface{}) bool {
	deploy, ok := svc["deploy"].(map[string]interface{})
	if !ok {
		return false
	}
	updateConfig, ok := deploy["update_config"].(map[string]interface{})
	if !ok {
		return false
	}
	order, ok := updateConfig["order"].(string)
	return ok && order == "start-first"
}

func composeHasProfile(composeData []byte, profile string) bool {
	profiles, err := compose.DiscoverProfiles(composeData)
	if err != nil {
		return false
	}
	for _, p := range profiles {
		if p == profile {
			return true
		}
	}
	return false
}

// applyProfileFilter applies the active --profiles / --services / default-profile
// filter to composeData and returns the filtered result. This must be called
// before any operation that should only concern the active portion of the stack
// (e.g. secret setup, validation).
func applyProfileFilter(composeData []byte) ([]byte, error) {
	switch {
	case len(profiles) > 0:
		return compose.FilterByProfile(composeData, profiles)
	case len(services) > 0:
		return compose.FilterServices(composeData, services)
	default:
		return compose.FilterByProfile(composeData, nil)
	}
}

func hasClipboardSupport() bool {
	for _, cmd := range []string{"wl-copy", "xclip", "xsel", "pbcopy", "clip"} {
		if _, err := exec.LookPath(cmd); err == nil {
			return true
		}
	}
	return false
}

func copyToClipboard(value string) error {
	candidates := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"pbcopy"},
		{"clip"},
	}

	var lastErr error
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate[0]); err != nil {
			continue
		}
		cmd := exec.Command(candidate[0], candidate[1:]...)
		cmd.Stdin = strings.NewReader(value)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no clipboard command available")
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

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/audit"
	"github.com/dargstack/dargstack/v4/internal/compose"
	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/prompt"
	"github.com/dargstack/dargstack/v4/internal/resource"
	"github.com/dargstack/dargstack/v4/internal/secret"
	"github.com/dargstack/dargstack/v4/internal/tls"
)

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
		domains := uniqueSortedDomains(tls.ExtractDomains(composeData, cfg.Development.Domain), cfg.Development.Certificate.Domains)
		printInfo(fmt.Sprintf("[dry-run] Step 4: TLS domains discovered: %s", strings.Join(domains, ", ")))
	}

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
		domains := uniqueSortedDomains(tls.ExtractDomains(composeData, cfg.Development.Domain), cfg.Development.Certificate.Domains)
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
	case deployAll:
		printInfo("Deploying full stack (--all: profile and service filters bypassed)")
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
				printInfo("Default profile detected: deploying only services in profile \"default\". Use --profiles, --services, --unlabeled or --all to change the set of deployed services.")
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

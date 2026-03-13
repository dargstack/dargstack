package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/swarm"
	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/compose"
	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/prompt"
	"github.com/dargstack/dargstack/v4/internal/secret"
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
- Includes production-only services`,
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

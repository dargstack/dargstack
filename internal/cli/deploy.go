package cli

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/swarm"
	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/compose"
	"github.com/dargstack/dargstack/v4/internal/docker"
)

var (
	production bool
	profiles   []string
	services   []string
	deployTag  string
	dryRun     bool

	listProfiles bool
	secretsOnly  bool
	redeployFlag bool
	deployAll    bool
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
	deployCmd.Flags().StringSliceVarP(&services, "services", "s", nil, "deploy only these services (comma-separated)")
	deployCmd.Flags().StringVarP(&deployTag, "tag", "t", "", "deploy a specific git tag (production only)")
	deployCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "trace all steps without deploying")
	deployCmd.Flags().BoolVarP(&redeployFlag, "re", "r", false, "remove the running stack before deploying")
	deployCmd.Flags().BoolVarP(&deployAll, "all", "a", false, "deploy the full stack ignoring --profiles and --services filters")
	deployCmd.Flags().BoolVar(&listProfiles, "list-profiles", false, "list discovered deploy profiles and exit")
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

	if listProfiles {
		return runDeployListMode()
	}

	if secretsOnly {
		return runSecretsOnly()
	}

	// --re: remove the running stack before deploying.
	if redeployFlag && !dryRun {
		if err := runRm(cmd, nil); err != nil {
			return fmt.Errorf("pre-deploy remove: %w", err)
		}
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

	discoveredProfiles, profErr := compose.DiscoverProfiles(composeData)
	if profErr != nil {
		return profErr
	}
	sort.Strings(discoveredProfiles)
	if len(discoveredProfiles) == 0 {
		printInfo("No profiles found")
	} else {
		printInfo("Discovered profiles:")
		for _, p := range discoveredProfiles {
			fmt.Printf("- %s\n", p)
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

package cli

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types/swarm"
	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/logger"
)

var (
	deployAll   bool
	deployTag   string
	forceDeploy bool
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the stack",
	Long: `Deploy services to a Docker Swarm stack.

By default, deploys to the development environment. This includes:
- Auto-building images for services with ` + "`dargstack.development.build`" + ` labels (controlled by ` + "`behavior.build.mode`" + `)
- Generating TLS certificates for local development
- Setting up secrets interactively or with defaults
- Validating all stack resources

Use ` + "`--environment production`" + ` to deploy to production, which:
- Requires all environment variables and secrets to be set
- Blocks deployment if default insecure secrets are present
- Includes production-only services`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().BoolVarP(&deployAll, "all", "a", false, "deploy the full stack ignoring --profiles and --services filters")
	deployCmd.Flags().BoolVar(&forceDeploy, "force", false, "remove the running stack before deploying")
	deployCmd.Flags().StringVarP(&deployTag, "tag", "t", "", "deploy a specific git tag (production only)")
}

func runDeploy(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()

	if dryRun {
		logger.L.Info(fmt.Sprintf("[dry-run] Tracing %s deployment for stack %q", env, cfg.Name))
	}

	if forceDeploy && !dryRun {
		if err := runRemove(cmd, nil); err != nil {
			return fmt.Errorf("pre-deploy remove: %w", err)
		}
	}

	if dryRun {
		return runDeployWithExecutor(ctx, cmd, nil, nil, env, true)
	}

	// Docker prerequisite check — create executor first so sudo is pre-warmed
	// before any Docker socket access.
	executor, err := docker.NewExecutor(cfg.Sudo)
	if err != nil {
		return wrapWithBugHint(err)
	}

	// When sudo is needed the Docker SDK cannot reach the socket directly,
	// so perform all pre-flight checks through the CLI executor.
	if executor.NeedsSudo() {
		if err := executor.Ping(); err != nil {
			return hintErr(
				fmt.Errorf("%s: %w", ErrDockerNotRunning, err),
				HintStartDocker,
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

		return runDeployWithExecutor(ctx, cmd, nil, executor, env, false)
	}

	// No sudo required — use the SDK for richer checks.
	dockerClient, err := docker.NewClient()
	if err != nil {
		return wrapWithBugHint(err)
	}
	defer func() { _ = dockerClient.Close() }()

	if err := dockerClient.Ping(ctx); err != nil {
		return hintErr(
			fmt.Errorf("%s: %w", ErrDockerNotRunning, err),
			HintStartDocker,
		)
	}

	// Swarm check
	swarmState, err := dockerClient.SwarmStatus(ctx)
	if err != nil {
		return wrapWithBugHint(err)
	}

	if swarmState != swarm.LocalNodeStateActive {
		if err := ensureSwarm(executor); err != nil {
			return err
		}
	}

	return runDeployWithExecutor(ctx, cmd, dockerClient, executor, env, false)
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

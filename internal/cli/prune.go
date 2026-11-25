package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/audit"
	"github.com/dargstack/dargstack/v4/internal/compose"
	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/prompt"
)

const pruneQuestion = "Remove stopped containers and unused images on all swarm nodes now?"

// otherNodesPruneTimeout is generous because removing many images on a slow disk takes minutes.
const otherNodesPruneTimeout = 10 * time.Minute

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove stopped containers and unused images on all swarm nodes",
	Long: `Remove stopped containers and unused images on all swarm nodes.

Runs the same cleanup that ` + "`dargstack deploy`" + ` offers after a production deploy, without deploying.
Stopped containers are removed first, so images only they referenced count as unused.
All images not used by a container are removed, not only dangling ones, except those the stack's last deployment for the selected environment references.
Those are kept on every node, so the pre-pull of a deploy is not undone and the next deploy does not download them again.
When dargstack runs on a swarm manager, the other nodes are pruned too, through a one-off global job running ` + "`docker:cli`" + ` with the node's Docker socket mounted.
Volumes are never touched: use ` + "`dargstack remove --volumes`" + ` for those.
Asks for confirmation unless ` + "`--no-interaction`" + ` is set.`,
	Args: cobra.NoArgs,
	RunE: runPrune,
}

func runPrune(_ *cobra.Command, _ []string) error {
	if dryRun {
		logger.L.Info("[dry-run] Would remove stopped containers and unused images on all swarm nodes")
		return nil
	}

	if !noInteraction {
		if ok, err := confirmFunc(pruneQuestion, false); err != nil || !ok {
			return err
		}
	}

	executor, err := docker.NewExecutor(string(cfg.Runtime.Sudo))
	if err != nil {
		return err
	}
	return pruneRuntime(executor, cfg.Metadata.Name, lastDeployedImages())
}

// lastDeployedImages returns the images of the latest deployment snapshot, or none when there is no such snapshot.
func lastDeployedImages() []string {
	dep, err := audit.LatestDeployment(audit.AuditLogDir(stackDir), env)
	if err == nil {
		var data []byte
		if data, err = audit.LoadDeployment(dep.Path); err == nil {
			return compose.ExtractServiceImages(data)
		}
	}
	logger.L.Warn(fmt.Sprintf("No previous %s deployment found: keeping no stack images", env))
	return nil
}

// offerRuntimeCleanup only warns on failure, as the deploy before it already succeeded.
func offerRuntimeCleanup(executor *docker.Executor, keep []string) {
	if ok, err := confirmFunc(pruneQuestion, false); err != nil || !ok {
		return
	}
	if err := pruneRuntime(executor, cfg.Metadata.Name, keep); err != nil {
		logger.L.Warn(err.Error())
	}
}

// pruneRuntime removes stopped containers and then unused images except those in keep, on this node and on the other swarm nodes.
func pruneRuntime(executor *docker.Executor, stack string, keep []string) error {
	var containerOut string
	var removed int
	var containerErr, imageErr error

	_ = prompt.Spin("Cleaning up", func(context.Context) error {
		containerOut, containerErr = executor.Run("container", "prune", "-f")
		if containerErr != nil {
			return nil
		}
		removed, imageErr = docker.PruneImagesKeeping(executor, keep)
		return nil
	})

	if containerErr != nil {
		return fmt.Errorf("container cleanup failed: %w", containerErr)
	}
	if imageErr != nil {
		return fmt.Errorf("image cleanup failed: %w", imageErr)
	}

	logger.Success(fmt.Sprintf(
		"Cleanup complete. Containers: %s | Images: removed %d",
		strings.TrimSpace(containerOut),
		removed,
	))

	var pruned, failures []string
	var nodesErr error
	_ = prompt.Spin("Cleaning up other swarm nodes", func(context.Context) error {
		pruned, failures, nodesErr = docker.PruneOtherNodes(executor, stack, keep, otherNodesPruneTimeout)
		return nil
	})
	if nodesErr != nil {
		return fmt.Errorf("swarm node cleanup failed: %w", nodesErr)
	}
	if len(pruned) > 0 {
		logger.Success("Cleaned up swarm node(s): " + strings.Join(pruned, ", "))
	}
	for _, f := range failures {
		logger.L.Error(f)
	}
	if len(failures) > 0 {
		return fmt.Errorf("cleanup failed on one or more swarm nodes")
	}
	return nil
}

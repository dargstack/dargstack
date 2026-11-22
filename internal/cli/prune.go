package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/prompt"
)

const pruneQuestion = "Remove stopped containers and unused images now?"

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove stopped containers and unused images",
	Long: `Remove stopped containers and unused images.

Runs the same cleanup that ` + "`dargstack deploy`" + ` offers after a production deploy, without deploying.
Stopped containers are removed first, so images only they referenced count as unused.
All images not used by a container are removed, not only dangling ones.
Volumes are never touched: use ` + "`dargstack remove --volumes`" + ` for those.
Asks for confirmation unless ` + "`--no-interaction`" + ` is set.`,
	Args: cobra.NoArgs,
	RunE: runPrune,
}

func runPrune(_ *cobra.Command, _ []string) error {
	if dryRun {
		logger.L.Info("[dry-run] Would remove stopped containers and unused images")
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
	return pruneRuntime(executor)
}

// offerRuntimeCleanup only warns on failure, as the deploy before it already succeeded.
func offerRuntimeCleanup(executor *docker.Executor) {
	if ok, err := confirmFunc(pruneQuestion, false); err != nil || !ok {
		return
	}
	if err := pruneRuntime(executor); err != nil {
		logger.L.Warn(err.Error())
	}
}

// pruneRuntime removes stopped containers and then unused images.
func pruneRuntime(executor *docker.Executor) error {
	var containerOut, imageOut string
	var containerErr, imageErr error

	_ = prompt.Spin("Cleaning up", func(context.Context) error {
		containerOut, containerErr = executor.Run("container", "prune", "-f")
		if containerErr != nil {
			return nil
		}
		imageOut, imageErr = executor.Run("image", "prune", "-af")
		return nil
	})

	if containerErr != nil {
		return fmt.Errorf("container cleanup failed: %w", containerErr)
	}
	if imageErr != nil {
		return fmt.Errorf("image cleanup failed: %w", imageErr)
	}

	logger.Success(fmt.Sprintf(
		"Cleanup complete. Containers: %s | Images: %s",
		strings.TrimSpace(containerOut),
		strings.TrimSpace(imageOut),
	))
	return nil
}

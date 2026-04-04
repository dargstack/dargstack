package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/prompt"
)

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

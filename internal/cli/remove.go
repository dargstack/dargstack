package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/internal/compose"
	"github.com/dargstack/dargstack/internal/docker"
	"github.com/dargstack/dargstack/internal/prompt"
)

var removeVolumes bool

var rmCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"rm"},
	Short:   "Remove the deployed stack",
	Long: `Remove the deployed stack.

Removes all services, networks, and secrets from the Docker Swarm stack.
Use --profile or --services to remove only a subset of services. Without
those flags the full stack is removed. Use --production to build the compose
from production sources when resolving which services belong to a profile.
Optionally (with --volumes) removes all stack volumes, clearing persistent data.`,
	RunE: runRm,
}

func init() {
	rmCmd.Flags().BoolVar(&removeVolumes, "volumes", false, "also remove stack volumes")
	rmCmd.Flags().BoolVar(&production, "production", false, "remove in production mode")
	rmCmd.Flags().StringVar(&profile, "profile", "", "remove only services in this compose profile")
	rmCmd.Flags().StringSliceVar(&services, "services", nil, "remove only the specified services")
}

func runRm(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	executor, err := docker.NewExecutor(cfg.Sudo)
	if err != nil {
		return err
	}

	// When sudo is needed the Docker SDK cannot reach the socket,
	// so use the CLI executor for all checks.
	var dockerClient *docker.Client
	if !executor.NeedsSudo() {
		dockerClient, err = docker.NewClient()
		if err != nil {
			return err
		}
		defer func() { _ = dockerClient.Close() }()
	}

	running := isStackRunning(ctx, dockerClient, executor)
	if !running {
		printInfo(fmt.Sprintf("Stack %q is not running", cfg.Name))
		return nil
	}

	// Targeted removal: only a profile or specific services.
	if profile != "" || len(services) > 0 {
		return runRmTargeted(executor)
	}

	if err := docker.StackRemove(executor, cfg.Name); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Stack %q removed", cfg.Name))

	if removeVolumes {
		if !noInteraction {
			ok, err := prompt.Confirm("Confirm removal of all stack volumes?", false)
			if err != nil || !ok {
				return nil
			}
		}

		volumes, err := docker.VolumeList(executor, cfg.Name)
		if err != nil {
			return fmt.Errorf("list volumes: %w", err)
		}
		if len(volumes) > 0 {
			if err := docker.VolumeRemove(executor, volumes); err != nil {
				return fmt.Errorf("remove volumes: %w", err)
			}
			printSuccess(fmt.Sprintf("Removed %d volume(s)", len(volumes)))
		}
	}

	return nil
}

// runRmTargeted removes only the services selected by --profile / --services.
func runRmTargeted(executor *docker.Executor) error {
	// Build compose to determine which services belong to the active profile.
	var composeData []byte
	var err error
	if production {
		composeData, err = buildProductionCompose()
	} else {
		composeData, err = buildDevelopmentCompose()
	}
	if err != nil {
		return fmt.Errorf("build compose for targeted remove: %w", err)
	}

	composeData, err = applyProfileFilter(composeData)
	if err != nil {
		return fmt.Errorf("filter compose by profile: %w", err)
	}

	// Extract service names from the filtered compose.
	targetServices, err := compose.ServiceNames(composeData)
	if err != nil {
		return fmt.Errorf("extract service names: %w", err)
	}

	// If --services was also provided, intersect with the compose-filtered set.
	if len(services) > 0 {
		svcSet := make(map[string]bool, len(services))
		for _, s := range services {
			svcSet[s] = true
		}
		filtered := targetServices[:0]
		for _, s := range targetServices {
			if svcSet[s] {
				filtered = append(filtered, s)
			}
		}
		targetServices = filtered
	}

	if len(targetServices) == 0 {
		printInfo("No matching services to remove")
		return nil
	}

	sort.Strings(targetServices)

	// Prefix with stack name for Docker service removal.
	fullNames := make([]string, len(targetServices))
	for i, s := range targetServices {
		fullNames[i] = cfg.Name + "_" + s
	}

	printInfo(fmt.Sprintf("Removing services: %s", strings.Join(targetServices, ", ")))
	if err := docker.ServiceRemove(executor, fullNames); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Removed %d service(s)", len(fullNames)))
	return nil
}

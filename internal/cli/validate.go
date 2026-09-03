package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/docker"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/resource"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate stack resources",
	Long: `Validate stack resources and configuration.

Checks:
- dargstack.yaml matches the JSON Schema
- All secrets files referenced in compose definitions exist
- All Dockerfile contexts for services with ` + "`dargstack.development.build`" + ` labels are present
- TLS certificates directory exists for development
- The merged compose output passes ` + "`docker compose config`" + ` (skipped if the compose CLI plugin isn't installed)`,
	RunE: runValidate,
}

func init() {}

// composeExecutor builds a best-effort docker Executor for `docker compose config` validation.
// It returns nil (rather than an error) when docker isn't available, so callers skip that check instead of failing outright, matching commands like `dargstack validate` that don't otherwise require Docker.
func composeExecutor() *docker.Executor {
	executor, err := docker.NewExecutor(string(cfg.Runtime.Sudo))
	if err != nil {
		logger.L.Debug(fmt.Sprintf("Skipping `docker compose config` validation: %v", err))
		return nil
	}
	return executor
}

func runValidate(cmd *cobra.Command, args []string) error {
	var composeData []byte
	var err error

	if isProduction() {
		composeData, err = buildProductionCompose()
	} else {
		composeData, err = buildDevelopmentCompose()
	}
	if err != nil {
		return err
	}

	composeData, filterMsg, err := applyProfileFilter(composeData)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrFilterComposeByProfile, err)
	}
	logger.L.Info(filterMsg)

	issues, err := resource.Validate(composeData, stackDir, isProduction())
	if err != nil {
		return err
	}

	composeErr := deployValidateComposeConfig(composeExecutor(), composeData)

	if len(issues) == 0 {
		if composeErr != nil {
			return composeErr
		}
		logger.Success("All resources are valid")
		return nil
	}

	logger.L.Info("Validating stack...")
	hasErrors := printIssues(issues)
	if composeErr != nil {
		return composeErr
	}
	if hasErrors {
		return errors.New(ErrValidationFailed)
	}

	return nil
}

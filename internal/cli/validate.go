package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/resource"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate stack resources",
	Long: `Validate stack resources and configuration.

Checks:
- All secrets files referenced in compose definitions exist
- All Dockerfile contexts for services with ` + "`dargstack.development.build`" + ` labels are present
- TLS certificates directory exists for development`,
	RunE: runValidate,
}

func init() {}

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

	if len(issues) == 0 {
		logger.Success("All resources are valid")
		return nil
	}

	if printIssues(issues) {
		return errors.New(ErrValidationFailed)
	}

	return nil
}

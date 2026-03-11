package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/internal/resource"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate stack resources",
	Long: `Validate stack resources and configuration.

Checks:
- All secrets files referenced in compose definitions exist
- All Dockerfile contexts for services with dargstack.development.build labels are present
- TLS certificates directory exists for development`,
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().BoolVar(&production, "production", false, "validate in production mode")
	validateCmd.Flags().StringSliceVar(&profiles, "profiles", nil, "activate one or more compose profiles; unlabeled services are included unless a 'default' profile is defined")
	validateCmd.Flags().StringSliceVar(&services, "services", nil, "validate specific services only")
}

func runValidate(cmd *cobra.Command, args []string) error {
	var composeData []byte
	var err error

	if production {
		composeData, err = buildProductionCompose()
	} else {
		composeData, err = buildDevelopmentCompose()
	}
	if err != nil {
		return err
	}

	composeData, err = applyProfileFilter(composeData)
	if err != nil {
		return fmt.Errorf("filter compose by profile: %w", err)
	}

	issues, err := resource.Validate(composeData, stackDir, production)
	if err != nil {
		return err
	}

	if len(issues) == 0 {
		printSuccess("All resources are valid")
		return nil
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
		return fmt.Errorf("validation failed")
	}

	printWarning(fmt.Sprintf("%d warning(s) found", len(issues)))
	return nil
}

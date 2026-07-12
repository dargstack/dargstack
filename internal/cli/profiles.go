package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/compose"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "List discovered deploy profiles",
	Long: `List profiles discovered from compose service definitions.

Profiles are defined via the dargstack.profiles label on services.
Use --environment production to list profiles from the production compose stack.`,
	RunE: runProfiles,
}

func runProfiles(_ *cobra.Command, _ []string) error {
	var composeData []byte
	var err error

	if env == "production" {
		composeData, err = buildProductionCompose()
	} else {
		composeData, err = buildDevelopmentCompose()
	}
	if err != nil {
		return wrapWithBugHint(err)
	}

	discovered, profErr := compose.DiscoverProfiles(composeData)
	if profErr != nil {
		return profErr
	}
	sort.Strings(discovered)
	if len(discovered) == 0 {
		printInfo("No profiles found")
	} else {
		printInfo("Discovered profiles:")
		for _, p := range discovered {
			fmt.Printf("- %s\n", p)
		}
	}

	return nil
}

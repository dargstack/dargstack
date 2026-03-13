package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dargstack/dargstack/v4/internal/compose"
	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/secret"
)

func buildDevelopmentCompose() ([]byte, error) {
	var paths []string

	// Service files (services-only architecture)
	svcFiles, err := config.CollectServiceFiles(config.DevDir(stackDir))
	if err != nil {
		return nil, err
	}
	paths = append(paths, svcFiles...)

	if len(paths) == 0 {
		return nil, hintErr(
			fmt.Errorf("no compose sources found"),
			fmt.Sprintf("Create service directories in %s, each containing a compose.yaml.", config.DevDir(stackDir)),
		)
	}

	var data []byte
	if len(paths) == 1 {
		data, err = compose.LoadSingle(paths[0])
	} else {
		data, err = compose.MergeFiles(paths...)
	}
	if err != nil {
		return nil, err
	}

	data, err = secret.RewriteSecretFilePaths(data, config.SecretsDir(stackDir))
	if err != nil {
		return nil, fmt.Errorf("rewrite secret file paths: %w", err)
	}

	return data, nil
}

func buildProductionCompose() ([]byte, error) {
	var paths []string

	// Dev sources (base layer)
	devSvcFiles, err := config.CollectServiceFiles(config.DevDir(stackDir))
	if err != nil {
		return nil, err
	}
	paths = append(paths, devSvcFiles...)

	// Production overlays
	prodSvcFiles, err := config.CollectServiceFiles(config.ProdDir(stackDir))
	if err != nil {
		return nil, err
	}
	paths = append(paths, prodSvcFiles...)

	if len(paths) == 0 {
		return nil, fmt.Errorf("no compose sources found")
	}

	merged, err := compose.MergeFiles(paths...)
	if err != nil {
		return nil, err
	}

	// Strip dev-only markers in production
	merged = compose.StripDevOnlyMarkers(merged)

	// Remap bind mounts from development paths to mirrored production paths
	// when production files/directories exist.
	if remapped, remapErr := compose.RewriteProductionBindMounts(
		merged,
		config.DevDir(stackDir),
		config.ProdDir(stackDir),
	); remapErr == nil {
		merged = remapped
	} else {
		printWarning(fmt.Sprintf("Failed to rewrite production bind mounts: %v", remapErr))
	}

	// Merge env files for production
	devEnv := config.DevEnvFile(stackDir)
	prodEnv := config.ProdEnvFile(stackDir)
	mergedEnv, mergeErr := compose.MergeEnvFiles(devEnv, prodEnv)
	if mergeErr != nil {
		printWarning(fmt.Sprintf("Failed to merge env files: %v", mergeErr))
	} else if len(mergedEnv) > 0 {
		envPath := filepath.Join(config.ArtifactsDir(stackDir), ".env.merged")
		if mkErr := os.MkdirAll(filepath.Dir(envPath), 0o755); mkErr == nil {
			_ = os.WriteFile(envPath, mergedEnv, 0o644)
		}
	}

	merged, err = secret.RewriteSecretFilePaths(merged, config.SecretsDir(stackDir))
	if err != nil {
		return nil, fmt.Errorf("rewrite secret file paths: %w", err)
	}

	return merged, nil
}

func composeHasProfile(composeData []byte, profile string) bool {
	profiles, err := compose.DiscoverProfiles(composeData)
	if err != nil {
		return false
	}
	for _, p := range profiles {
		if p == profile {
			return true
		}
	}
	return false
}

// applyProfileFilter applies the active --profiles / --services / default-profile
// filter to composeData and returns the filtered result. This must be called
// before any operation that should only concern the active portion of the stack
// (e.g. secret setup, validation).
func applyProfileFilter(composeData []byte) ([]byte, error) {
	if deployAll {
		return composeData, nil
	}
	switch {
	case len(profiles) > 0:
		return compose.FilterByProfile(composeData, profiles)
	case len(services) > 0:
		return compose.FilterServices(composeData, services)
	default:
		return compose.FilterByProfile(composeData, nil)
	}
}

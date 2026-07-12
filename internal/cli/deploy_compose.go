package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
			errors.New(ErrNoComposeSources),
			fmt.Sprintf("Create service directories in %s, each containing a compose.yaml.", config.DevDir(stackDir)),
		)
	}

	var data []byte
	if len(paths) == 1 {
		data, err = compose.LoadSingle(stackDir, paths[0])
	} else {
		data, err = compose.MergeFiles(stackDir, paths...)
	}
	if err != nil {
		return nil, err
	}

	data, err = secret.RewriteSecretFilePaths(data, config.SecretsDir(stackDir))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrRewriteSecretFilePaths, err)
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
		return nil, errors.New(ErrNoComposeSources)
	}

	// MergeFilesProduction strips # dargstack:dev-only markers from each source
	// file's raw bytes before YAML parsing, since YAML roundtrips discard comments.
	merged, err := compose.MergeFilesProduction(stackDir, paths...)
	if err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf("%s: %w", ErrRewriteSecretFilePaths, err)
	}

	return merged, nil
}

func buildComposeData(prod bool) ([]byte, error) {
	if prod {
		return buildProductionCompose()
	}
	return buildDevelopmentCompose()
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
// filter to composeData and returns the filtered result along with a human-readable
// description of the filter applied. This must be called before any operation that
// should only concern the active portion of the stack (e.g. secret setup, validation).
//
// In production mode without an explicit --profiles or --services flag, all
// services are included (matching the deploy step that skips the default-profile
// filter in production).
func applyProfileFilter(composeData []byte) (filtered []byte, msg string, err error) {
	if deployAll {
		return composeData, "Profile filter bypassed (--all)", nil
	}
	switch {
	case len(profiles) > 0:
		result, err := compose.FilterByProfile(composeData, profiles)
		return result, fmt.Sprintf("Filtering by profiles: %s", strings.Join(profiles, ", ")), err
	case len(services) > 0:
		result, err := compose.FilterServices(composeData, services)
		return result, fmt.Sprintf("Filtering by services: %s", strings.Join(services, ", ")), err
	case isProduction():
		return composeData, "Production mode: no profile filter applied", nil
	default:
		hasDefault := composeHasProfile(composeData, "default")
		result, err := compose.FilterByProfile(composeData, nil)
		if hasDefault {
			return result, "Default profile active: filtering to \"default\" profile only", err
		}
		return result, "No default profile: no filtering applied", err
	}
}

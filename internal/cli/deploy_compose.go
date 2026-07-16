package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dargstack/dargstack/v4/internal/compose"
	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/secret"
)

func buildDevelopmentCompose() ([]byte, error) {
	var paths []string

	// Service files (services-only architecture)
	svcFiles, err := config.CollectServiceFiles(cfg.DevDir())
	if err != nil {
		return nil, err
	}
	paths = append(paths, svcFiles...)

	if len(paths) == 0 {
		return nil, hintErr(
			errors.New(ErrNoComposeSources),
			fmt.Sprintf("Create service directories in %s, each containing a compose.yaml.", cfg.DevDir()),
		)
	}

	var data []byte
	if len(paths) == 1 {
		data, err = compose.LoadSingle(stackDir, getPlatform(), paths[0])
	} else {
		data, err = compose.MergeFiles(stackDir, getPlatform(), paths...)
	}
	if err != nil {
		return nil, err
	}

	data, err = secret.RewriteSecretFilePaths(data, cfg.SecretsDir())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrRewriteSecretFilePaths, err)
	}

	return data, nil
}

func buildProductionCompose() ([]byte, error) {
	var paths []string

	// Dev sources (base layer)
	devSvcFiles, err := config.CollectServiceFiles(cfg.DevDir())
	if err != nil {
		return nil, err
	}
	paths = append(paths, devSvcFiles...)

	// Production overlays
	prodSvcFiles, err := config.CollectServiceFiles(cfg.ProdDir())
	if err != nil {
		return nil, err
	}
	paths = append(paths, prodSvcFiles...)

	if len(paths) == 0 {
		return nil, errors.New(ErrNoComposeSources)
	}

	// MergeFilesProduction strips # dargstack:dev-only markers from each source
	// file's raw bytes before YAML parsing, since YAML roundtrips discard comments.
	merged, err := compose.MergeFilesProduction(stackDir, getPlatform(), paths...)
	if err != nil {
		return nil, err
	}

	// Convert file: secrets to external: true for production.
	if externalized, extErr := compose.RewriteProductionSecrets(merged); extErr == nil {
		merged = externalized
	} else {
		logger.L.Warn(fmt.Sprintf("Failed to rewrite production secrets: %v", extErr))
	}

	// Remap bind mounts from development paths to mirrored production paths
	// when production files/directories exist.
	if remapped, remapErr := compose.RewriteProductionBindMounts(
		merged,
		cfg.DevDir(),
		cfg.ProdDir(),
	); remapErr == nil {
		merged = remapped
	} else {
		logger.L.Warn(fmt.Sprintf("Failed to rewrite production bind mounts: %v", remapErr))
	}

	// Merge env files for production
	devEnv := cfg.DevEnvFile()
	prodEnv := cfg.ProdEnvFile()
	mergedEnv, mergeErr := compose.MergeEnvFiles(devEnv, prodEnv)
	if mergeErr != nil {
		logger.L.Warn(fmt.Sprintf("Failed to merge env files: %v", mergeErr))
	} else if len(mergedEnv) > 0 {
		envPath := filepath.Join(cfg.ArtifactsDir(), ".env.merged")
		if mkErr := os.MkdirAll(filepath.Dir(envPath), 0o755); mkErr == nil {
			_ = os.WriteFile(envPath, mergedEnv, 0o644)
		}
	}

	merged, err = secret.RewriteSecretFilePaths(merged, cfg.SecretsDir())
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
		return composeData, "Deploying full stack (--all: profile and service filters bypassed)", nil
	}
	switch {
	case len(profiles) > 0:
		result, err := compose.FilterByProfile(composeData, profiles)
		return result, fmt.Sprintf("Deploying with profiles %v active", profiles), err
	case len(services) > 0:
		result, err := compose.FilterServices(composeData, services)
		return result, fmt.Sprintf("Deploying services: %s", strings.Join(services, ", ")), err
	case isProduction():
		return composeData, "Production deployment: deploying all services", nil
	default:
		hasDefault := composeHasProfile(composeData, "default")
		result, err := compose.FilterByProfile(composeData, nil)
		if hasDefault {
			return result, "Deploying services in profile \"default\". Use --profiles, --services, --unlabeled or --all to change the set of deployed services.", err
		}
		return result, "No default profile detected: deploying all services", err
	}
}

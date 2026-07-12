package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/dargstack/dargstack/v4/internal/compose"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/prompt"
)

// applyEnvToProcess loads the resolved .env values and STACK_DOMAIN into the
// current process environment so that `docker stack deploy -c -` can
// interpolate them. The process environment takes precedence — existing values
// are not overwritten.
// It returns the map of vars that were applied so the caller can forward them
// explicitly to sudo subprocesses via Executor.SetComposeEnv.
func applyEnvToProcess(prod bool) map[string]string {
	env, err := compose.LoadEnvFile(cfg.DevEnvFile())
	if err != nil {
		env = map[string]string{}
	}
	if prod {
		if prodEnv, pErr := compose.LoadEnvFile(cfg.ProdEnvFile()); pErr == nil {
			for k, v := range prodEnv {
				env[k] = v
			}
		}
	}
	applied := make(map[string]string, len(env)+1)
	for k, v := range env {
		if v != "" && os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
		if v != "" {
			applied[k] = v
		}
	}
	stackDomain := cfg.Environment.Development.Domain
	if prod {
		stackDomain = cfg.Environment.Production.Domain
	}
	_ = os.Setenv("STACK_DOMAIN", stackDomain)
	applied["STACK_DOMAIN"] = stackDomain
	return applied
}

func promptForEnvValues(prod bool) error {
	envPath := cfg.DevEnvFile()
	env, err := compose.LoadEnvFile(envPath)
	if err != nil {
		return nil // no env file is fine
	}

	if prod {
		// For production, also load prod env
		prodEnv, prodErr := compose.LoadEnvFile(cfg.ProdEnvFile())
		if prodErr == nil {
			for k, v := range prodEnv {
				env[k] = v
			}
		}
	}

	missing := compose.FindMissingEnvValues(env)
	if len(missing) == 0 {
		return nil
	}

	if prod {
		return hintErr(
			fmt.Errorf("production deployment requires all environment variables to be set — missing: %s", strings.Join(missing, ", ")),
			fmt.Sprintf("Add the missing values to %s.", cfg.ProdEnvFile()),
		)
	}

	if noInteraction {
		logger.L.Warn(fmt.Sprintf("Missing environment variable values: %s", strings.Join(missing, ", ")))
		return nil
	}

	logger.L.Info(fmt.Sprintf("Found %d environment variable(s) without values", len(missing)))
	ok, promptErr := prompt.Confirm("Fill in missing environment variable values now?", true)
	if promptErr != nil || !ok {
		return nil
	}

	for _, key := range missing {
		val, inputErr := prompt.Input(fmt.Sprintf("Value for %s:", key), "")
		if inputErr != nil {
			return inputErr
		}
		if val != "" {
			env[key] = val
		}
	}

	return compose.WriteEnvFile(envPath, env)
}

package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/platform"
	"github.com/dargstack/dargstack/v4/internal/prompt"
	"github.com/dargstack/dargstack/v4/internal/update"
	"github.com/dargstack/dargstack/v4/internal/version"
)

// selfUpdateFunc performs a self-update; overridden in tests.
var selfUpdateFunc = update.SelfUpdate

// confirmFunc asks the user a yes/no question; overridden in tests.
var confirmFunc = prompt.Confirm

var (
	cfgPath       string
	dryRun        bool
	env           string
	noInteraction bool
	offline       bool
	outputFormat  string
	platformFlag  string
	profiles      []string
	services      []string
	verbose       bool

	cfg      *config.Config
	stackDir string

	// stackDomainExplicit records whether STACK_DOMAIN was set in the
	// environment before dargstack ran, so applyStackDomainDefault never
	// overrides a user-supplied value.
	stackDomainExplicit bool
)

const (
	bugReportURL   = "https://github.com/dargstack/dargstack/issues/new?template=bug_report.yaml"
	discussionsURL = "https://github.com/dargstack/dargstack/discussions"
)

var logLevel string

var rootCmd = &cobra.Command{
	Use:          "dargstack",
	Short:        "Docker stack helper CLI",
	Long:         "dargstack - simplified, approachable Docker Swarm stack management.",
	Version:      fmt.Sprintf("%s (commit: %s, built: %s)", version.Version, version.Commit, version.Date),
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		resolveProfiles()

		// Propagate --no-interaction to the prompt package.
		prompt.NonInteractive = noInteraction

		// Set log level from flag. --verbose overrides to debug.
		if verbose {
			logger.Level.Set(slog.LevelDebug)
		} else {
			switch logLevel {
			case "error":
				logger.Level.Set(slog.LevelError)
			case "warn":
				logger.Level.Set(slog.LevelWarn)
			case "debug":
				logger.Level.Set(slog.LevelDebug)
			default:
				logger.Level.Set(slog.LevelInfo)
			}
		}

		// Start the update check for all commands except meta-commands where
		// a background network call would be inappropriate.
		if !offline && !isUpdateSkippedCommand(cmd) {
			update.BackgroundCheck()
		}

		// Skip config loading for commands that don't need a stack project.
		// Walk up to the first subcommand (child of root) to check.
		if isSkippedCommand(cmd) {
			return nil
		}

		var err error
		if cfgPath != "" {
			abs, absErr := filepath.Abs(cfgPath)
			if absErr != nil {
				return fmt.Errorf("resolve config path: %w", absErr)
			}
			stackDir = abs
		} else {
			stackDir, err = config.DetectStackDir()
			if err != nil {
				return hintErr(
					fmt.Errorf("not in a dargstack project: %w", err),
					"Run `dargstack init` to bootstrap a new project, or `cd` into an existing one.",
				)
			}
		}

		stackDomainExplicit = os.Getenv("STACK_DOMAIN") != ""

		cfg, err = config.Load(stackDir)
		if err != nil {
			return resolveVersionIncompatibility(err)
		}

		applyStackDomainDefault()

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		result := update.CollectBackgroundCheck()
		update.PrintUpdateNotice(result)
	},
}

// applyStackDomainDefault sets STACK_DOMAIN from cfg unless the user
// explicitly set it in the environment. It uses the domain matching the
// active --environment: production commands use the production domain,
// development commands use the development domain. Called once from
// PersistentPreRunE, and again by production deploy after checkoutDeployTag
// reloads cfg, so the env var reflects whichever dargstack.yaml is actually
// on disk.
func applyStackDomainDefault() {
	if stackDomainExplicit {
		return
	}
	domain := cfg.Environment.Development.Domain
	if env == "production" {
		domain = cfg.Environment.Production.Domain
	}
	_ = os.Setenv("STACK_DOMAIN", domain)
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "configuration", "c", "", "path to stack directory (default: auto-detect)")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "d", false, "trace all steps without executing")
	rootCmd.PersistentFlags().StringVarP(&env, "environment", "e", "development", "environment to operate on: development|production")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "format", "f", "table", "output format for compatible commands: table|json")
	rootCmd.PersistentFlags().BoolVarP(&noInteraction, "no-interaction", "n", false, "disable interactive prompts")
	rootCmd.PersistentFlags().BoolVarP(&offline, "offline", "o", false, "skip fetching remote resources")
	rootCmd.PersistentFlags().StringVar(&platformFlag, "platform", "", "target platform for compose overrides (default: auto-detect)")
	rootCmd.PersistentFlags().StringSliceVarP(&profiles, "profiles", "p", nil, FlagDescProfiles)
	rootCmd.PersistentFlags().StringSliceVarP(&services, "services", "s", nil, "filter to specific services")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	_ = rootCmd.PersistentFlags().MarkHidden("verbose")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "log level: error, warn, info, debug")

	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(certificatesCmd)
	rootCmd.AddCommand(cloneCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(docsCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(profilesCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(secretCmd)
	rootCmd.AddCommand(schemaCmd)
	rootCmd.AddCommand(skillCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(validateCmd)
}

// Root returns the root command for use by external tools such as doc generators.
func Root() *cobra.Command { return rootCmd }

// isUpdateSkippedCommand returns true for meta-commands where a background
// network call for an update check would be inappropriate.
func isUpdateSkippedCommand(cmd *cobra.Command) bool {
	skipped := map[string]bool{
		"completion": true,
		"help":       true,
		"skill":      true,
		"update":     true,
	}
	for c := cmd; c != nil; c = c.Parent() {
		if skipped[c.Name()] {
			return true
		}
	}
	return false
}

// isSkippedCommand returns true if the command (or its nearest non-root ancestor)
// is one that doesn't require a stack project directory.
func isSkippedCommand(cmd *cobra.Command) bool {
	skipped := map[string]bool{
		"clone":      true,
		"completion": true,
		"help":       true,
		"init":       true,
		"initialize": true,
		"schema":     true,
		"skill":      true,
		"update":     true,
	}
	// Walk up from the leaf command to the first child of root.
	for c := cmd; c != nil; c = c.Parent() {
		if skipped[c.Name()] {
			return true
		}
	}
	return false
}

// resolveVersionIncompatibility offers to self-update when the CLI version
// does not satisfy the project's compatibility constraint, then asks the
// user to re-run the command against the new binary. Returns the original
// error unchanged when no update was offered or performed (offline mode,
// non-interactive mode, user declined, or the update itself failed).
func resolveVersionIncompatibility(err error) error {
	if offline {
		return err
	}
	var incompatErr *config.IncompatibleVersionError
	if !errors.As(err, &incompatErr) {
		return err
	}

	ok, _ := confirmFunc(fmt.Sprintf("%s Update dargstack now?", incompatErr.Error()), false)
	if !ok {
		return err
	}

	if updErr := selfUpdateFunc(); updErr != nil {
		return fmt.Errorf("%w (self-update failed: %v)", err, updErr)
	}
	return errors.New("dargstack was updated — please re-run the command")
}

// isProduction returns true if the active --environment is "production".
func isProduction() bool { return env == "production" }

// getPlatform returns the target platform for compose overrides.
// Uses --platform flag if set, otherwise auto-detects via runtime.GOOS.
func getPlatform() string {
	return platform.Get(platformFlag)
}

// resolveProfiles reads COMPOSE_PROFILES env var and populates the profiles
// variable when the --profiles flag was not used.
func resolveProfiles() {
	if profiles != nil {
		return
	}
	if raw := os.Getenv("COMPOSE_PROFILES"); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				profiles = append(profiles, p)
			}
		}
	}
}

// wrapWithBugHint wraps an error with a hint to report bugs or ask for help.
func wrapWithBugHint(err error) error {
	return fmt.Errorf("%w\n\n  If this is unexpected, please report a bug: %s\n  Or start a discussion: %s", err, bugReportURL, discussionsURL)
}

// hintErr prints a fix suggestion, then returns the error.
// This keeps error strings Go-conventional while still giving the user guidance.
// Unlike regular log messages, hints always print regardless of log level.
func hintErr(err error, suggestion string) error {
	fmt.Fprintln(os.Stderr, logger.StyleInfo.Render(suggestion))
	return err
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

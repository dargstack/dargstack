package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/prompt"
	"github.com/dargstack/dargstack/v4/internal/update"
	"github.com/dargstack/dargstack/v4/internal/version"
)

var (
	cfgPath       string
	dryRun        bool
	env           string
	noInteraction bool
	offline       bool
	outputFormat  string
	profiles      []string
	services      []string
	verbose       bool

	cfg      *config.Config
	stackDir string

	styleErr  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleOK   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleWarn = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

var rootCmd = &cobra.Command{
	Use:          "dargstack",
	Short:        "Docker stack helper CLI",
	Long:         "dargstack - simplified, approachable Docker Swarm stack management.",
	Version:      fmt.Sprintf("%s (commit: %s, built: %s)", version.Version, version.Commit, version.Date),
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Propagate --no-interaction to the prompt package.
		prompt.NonInteractive = noInteraction

		// Skip config loading for commands that don't need a stack project.
		// Walk up to the first subcommand (child of root) to check.
		if isSkippedCommand(cmd) {
			return nil
		}

		update.BackgroundCheck()

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

		cfg, err = config.Load(stackDir)
		if err != nil {
			return err
		}

		if err := cfg.CheckCompatibility(); err != nil {
			return err
		}

		// Set STACK_DOMAIN if not already set — use the domain matching the
		// active --environment. Production commands use the production domain;
		// development commands use the development domain.
		if os.Getenv("STACK_DOMAIN") == "" {
			domain := cfg.Development.Domain
			if env == "production" {
				domain = cfg.Production.Domain
			}
			_ = os.Setenv("STACK_DOMAIN", domain)
		}

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		result := update.CollectBackgroundCheck()
		update.PrintUpdateNotice(result)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "configuration", "c", "", "path to stack directory (default: auto-detect)")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "d", false, "trace all steps without executing")
	rootCmd.PersistentFlags().StringVarP(&env, "environment", "e", "development", "environment to operate on: development|production")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "format", "f", "table", "output format for compatible commands: table|json")
	rootCmd.PersistentFlags().BoolVarP(&noInteraction, "no-interaction", "n", false, "disable interactive prompts")
	rootCmd.PersistentFlags().BoolVar(&offline, "offline", false, "skip fetching remote resources")
	rootCmd.PersistentFlags().StringSliceVar(&profiles, "profiles", nil, FlagDescProfiles)
	rootCmd.PersistentFlags().StringSliceVarP(&services, "services", "s", nil, "filter to specific services")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(certificatesCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(docsCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(profilesCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(secretCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(validateCmd)
}

// Root returns the root command for use by external tools such as doc generators.
func Root() *cobra.Command { return rootCmd }

// isSkippedCommand returns true if the command (or its nearest non-root ancestor)
// is one that doesn't require a stack project directory.
func isSkippedCommand(cmd *cobra.Command) bool {
	skipped := map[string]bool{
		"completion": true,
		"help":       true,
		"initialize": true,
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

// isProduction returns true if the active --environment is "production".
func isProduction() bool { return env == "production" }

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func printError(msg string) {
	fmt.Fprintln(os.Stderr, styleErr.Render("Error: "+msg))
}

func printWarning(msg string) {
	fmt.Fprintln(os.Stderr, styleWarn.Render("Warning: "+msg))
}

func printSuccess(msg string) {
	fmt.Println(styleOK.Render(msg))
}

func printInfo(msg string) {
	fmt.Println(styleInfo.Render(msg))
}

const bugReportURL = "https://github.com/dargstack/dargstack/issues/new?template=bug_report.yaml"
const discussionsURL = "https://github.com/dargstack/dargstack/discussions"

// wrapWithBugHint wraps an error with a hint to report bugs or ask for help.
func wrapWithBugHint(err error) error {
	return fmt.Errorf("%w\n\n  If this is unexpected, please report a bug: %s\n  Or start a discussion: %s", err, bugReportURL, discussionsURL)
}

// hintErr prints a fix suggestion, then returns the error.
// This keeps error strings Go-conventional while still giving the user guidance.
func hintErr(err error, suggestion string) error {
	fmt.Fprintln(os.Stderr, styleInfo.Render(suggestion))
	return err
}

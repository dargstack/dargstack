package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/giturl"
	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/prompt"
)

var cloneTarget string

var cloneCmd = &cobra.Command{
	Use:   "clone [url]",
	Short: "Clone an existing dargstack project",
	Long: `Clone an existing dargstack project from a Git URL.

Supports https://, git@, git://, and ssh:// URLs.
Without arguments, prompts for a Git URL.
By default, clones into a subdirectory of the current directory named after the repository.

Use --target to specify a different directory for the clone.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runClone,
}

func init() {
	cloneCmd.Flags().StringVar(&cloneTarget, "target", "", "target directory for the clone (default: inferred from URL)")
}

func runClone(cmd *cobra.Command, args []string) error {
	url := ""
	if len(args) > 0 {
		url = args[0]
	}

	if url == "" {
		if noInteraction {
			return fmt.Errorf("--no-interaction requires a url argument")
		}

		var err error
		url, err = prompt.Input("Git URL", "")
		if err != nil {
			return err
		}
	}

	if url == "" {
		return fmt.Errorf("git URL is required")
	}

	if !isGitURL(url) {
		return fmt.Errorf("%q does not appear to be a Git URL", url)
	}

	target := cloneTarget
	targetExplicit := cmd.Flags().Changed("target")
	if !targetExplicit {
		target = giturl.RepoNameFromURL(url)
	}

	if !targetExplicit && !noInteraction {
		result, err := prompt.Input("Clone into directory", target)
		if err != nil {
			return err
		}
		target = result
	}

	if target == "" {
		return fmt.Errorf("target directory is required")
	}

	displayTarget := target
	if !filepath.IsAbs(target) {
		displayTarget = "./" + target
	}

	logger.L.Info(fmt.Sprintf("Cloning %s into %s ...", url, displayTarget))
	gitCmd := exec.Command("git", "clone", url, target) // #nosec G204 — URL is user-supplied intentionally
	gitCmd.Stdout = os.Stdout
	gitCmd.Stderr = os.Stderr
	if err := gitCmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	logger.Success(fmt.Sprintf("Project cloned into %s", displayTarget))
	logger.L.Info("Run `cd` into the directory and then `dargstack deploy` to start.")
	return nil
}

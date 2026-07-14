package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/logger"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <url>",
	Short: "Clone an existing dargstack project",
	Long: `Clone an existing dargstack project from a Git URL.

Supports https://, git@, git://, and ssh:// URLs.
The repository is cloned into the current directory.`,
	Args: cobra.ExactArgs(1),
	RunE: runClone,
}

func runClone(cmd *cobra.Command, args []string) error {
	url := args[0]

	if !isGitURL(url) {
		return fmt.Errorf("%q does not appear to be a Git URL", url)
	}

	logger.L.Info(fmt.Sprintf("Cloning %s ...", url))
	gitCmd := exec.Command("git", "clone", url) // #nosec G204 — URL is user-supplied intentionally
	gitCmd.Stdout = os.Stdout
	gitCmd.Stderr = os.Stderr
	if err := gitCmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	logger.Success("Project cloned. Navigate into the directory and run `dargstack deploy`.")
	return nil
}

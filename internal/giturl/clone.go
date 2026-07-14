package giturl

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/dargstack/dargstack/v4/internal/logger"
)

// CloneWithFallback attempts to clone using the primary URL.
// If the primary is SSH and no SSH agent is available, it skips
// directly to HTTPS (if set). If the clone fails and a fallback
// URL is available, it retries with the fallback.
func CloneWithFallback(g GitURL, targetDir string) error {
	if !g.IsSet() {
		return fmt.Errorf("no git URL configured")
	}

	var primary, fallback string

	switch {
	case g.SSH != "" && g.HTTPS != "":
		if SSHAgentAvailable() {
			primary = g.SSH
			fallback = g.HTTPS
		} else {
			primary = g.HTTPS
		}
	case g.SSH != "":
		primary = g.SSH
	default:
		primary = g.HTTPS
	}

	err := runClone(primary, targetDir)
	if err == nil {
		return nil
	}

	if fallback == "" || fallback == primary {
		return fmt.Errorf("clone %s: %w", primary, err)
	}

	logger.L.Warn(fmt.Sprintf("Clone via %s failed, falling back to %s", primary, fallback))
	if err := os.RemoveAll(targetDir); err != nil {
		logger.L.Warn(fmt.Sprintf("Failed to remove partial clone: %v", err))
	}

	err = runClone(fallback, targetDir)
	if err != nil {
		return fmt.Errorf("clone %s: %w", fallback, err)
	}
	return nil
}

func runClone(url, targetDir string) error {
	logger.L.Info(fmt.Sprintf("Cloning %s", url))
	cmd := exec.Command("git", "clone", "--depth", "1", url, targetDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

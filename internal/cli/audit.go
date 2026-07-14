package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/audit"
	"github.com/dargstack/dargstack/v4/internal/logger"
)

var (
	auditDiff bool
	auditEnv  string
	auditList bool
)

var auditCmd = &cobra.Command{
	Use:   "audit [timestamp]",
	Short: "View deployment audit log",
	Long: `View the final composed YAML that was deployed.

Without arguments, shows the latest deployment.`,
	RunE: runAudit,
}

func init() {
	auditCmd.Flags().BoolVar(&auditDiff, "difference", false, "show diff between current and last deployed")
	auditCmd.Flags().StringVar(&auditEnv, "environment", "development", "environment to audit (development or production)")
	auditCmd.Flags().BoolVar(&auditList, "list", false, "list all past deployments")
}

func runAudit(cmd *cobra.Command, args []string) error {
	auditDir := audit.AuditLogDir(stackDir)

	if auditList {
		return listDeployments(auditDir)
	}

	if len(args) > 0 {
		// Load specific deployment by matching timestamp prefix
		return showDeploymentByPrefix(auditDir, args[0])
	}

	// Show latest deployment
	dep, err := audit.LatestDeployment(auditDir, auditEnv)
	if err != nil {
		logger.L.Info("No previous deployments found — run `dargstack deploy` first to create a deployment snapshot.")
		return nil
	}

	data, err := audit.LoadDeployment(dep.Path)
	if err != nil {
		return err
	}

	if auditDiff {
		return showDiff(auditDir, dep, data)
	}

	fmt.Printf("# Latest %s deployment (%s)\n\n", dep.Environment, dep.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	fmt.Print(string(data))
	return nil
}

func listDeployments(auditDir string) error {
	deployments, err := audit.ListDeployments(auditDir)
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		logger.L.Info("No deployments found. Deploy first, then run `dargstack audit --list`.")
		return nil
	}

	fmt.Printf("%-24s  %-14s  %s\n", "TIMESTAMP", "ENVIRONMENT", "PATH")
	fmt.Printf("%-24s  %-14s  %s\n", strings.Repeat("-", 24), strings.Repeat("-", 14), strings.Repeat("-", 40))

	for _, d := range deployments {
		fmt.Printf("%-24s  %-14s  %s\n",
			d.Timestamp.Format("2006-01-02 15:04:05 UTC"),
			d.Environment,
			d.Path,
		)
	}
	return nil
}

func showDeploymentByPrefix(auditDir, prefix string) error {
	deployments, err := audit.ListDeployments(auditDir)
	if err != nil {
		return err
	}

	for _, d := range deployments {
		ts := d.Timestamp.Format("20060102T150405.000Z")
		if strings.HasPrefix(ts, prefix) {
			data, loadErr := audit.LoadDeployment(d.Path)
			if loadErr != nil {
				return loadErr
			}
			fmt.Printf("# %s deployment (%s)\n\n", d.Environment, d.Timestamp.Format("2006-01-02 15:04:05 UTC"))
			fmt.Print(string(data))
			return nil
		}
	}

	return hintErr(
		fmt.Errorf("no deployment matching prefix %q found", prefix),
		"Run `dargstack audit --list` to see all available deployments.",
	)
}

func showDiff(auditDir string, latest *audit.Deployment, latestData []byte) error {
	// Build current compose for comparison
	var currentData []byte
	var err error

	if latest.Environment == "production" {
		currentData, err = buildProductionCompose()
	} else {
		currentData, err = buildDevelopmentCompose()
	}
	if err != nil {
		return fmt.Errorf("build current compose for diff: %w", err)
	}

	// Write temp files for diff
	tmpDir, err := os.MkdirTemp("", "dargstack-diff-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	deployedPath := filepath.Join(tmpDir, "deployed.yaml")
	currentPath := filepath.Join(tmpDir, "current.yaml")
	if err := os.WriteFile(deployedPath, latestData, 0o644); err != nil {
		return fmt.Errorf("write deployed compose for diff: %w", err)
	}
	if err := os.WriteFile(currentPath, currentData, 0o644); err != nil {
		return fmt.Errorf("write current compose for diff: %w", err)
	}

	fmt.Printf("# Diff: last deployed vs current (%s)\n\n", latest.Environment)

	// Delegate to system diff for a proper unified diff output.
	// diff exits 0 (no diff), 1 (differ), or 2 (error). Both 0 and 1 produce
	// valid output; only exit code 2 indicates a real failure.
	deployedLabel := "deployed (" + latest.Timestamp.Format("2006-01-02 15:04:05 UTC") + ")"
	out, runErr := exec.Command("diff", "-u",
		"--label", deployedLabel,
		"--label", "current",
		deployedPath, currentPath).Output()

	var exitErr *exec.ExitError
	if runErr != nil && (!errors.As(runErr, &exitErr) || exitErr.ExitCode() == 2) {
		// diff binary unavailable or hard error — fall back to summary
		deployedLines := strings.Split(string(latestData), "\n")
		currentLines := strings.Split(string(currentData), "\n")
		if strings.Join(deployedLines, "\n") == strings.Join(currentLines, "\n") {
			logger.Success("No changes — current compose matches last deployed version")
			return nil
		}
		logger.L.Warn("diff not available; showing full current compose:")
		fmt.Print(string(currentData))
		return nil
	}

	if len(out) == 0 {
		logger.Success("No changes — current compose matches last deployed version")
		return nil
	}

	fmt.Print(string(out))
	return nil
}

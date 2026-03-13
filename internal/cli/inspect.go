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
)

var (
	inspectList bool
	inspectDiff bool
	inspectEnv  string
)

var inspectCmd = &cobra.Command{
	Use:   "inspect [timestamp]",
	Short: "Inspect deployed compose snapshots",
	Long: `Inspect the final composed YAML that was deployed.

Without arguments, shows the latest deployment.`,
	RunE: runInspect,
}

func init() {
	inspectCmd.Flags().BoolVarP(&inspectList, "list", "l", false, "list all past deployments")
	inspectCmd.Flags().BoolVarP(&inspectDiff, "diff", "d", false, "show diff between current and last deployed")
	inspectCmd.Flags().StringVarP(&inspectEnv, "env", "e", "development", "environment to inspect (development or production)")
}

func runInspect(cmd *cobra.Command, args []string) error {
	auditDir := audit.AuditLogDir(stackDir)

	if inspectList {
		return listDeployments(auditDir)
	}

	if len(args) > 0 {
		// Load specific deployment by matching timestamp prefix
		return showDeploymentByPrefix(auditDir, args[0])
	}

	// Show latest deployment
	dep, err := audit.LatestDeployment(auditDir, inspectEnv)
	if err != nil {
		return hintErr(
			fmt.Errorf("no previous deployments found"),
			"Run `dargstack deploy` first to create a deployment snapshot.",
		)
	}

	data, err := audit.LoadDeployment(dep.Path)
	if err != nil {
		return err
	}

	if inspectDiff {
		return showDiff(auditDir, dep, data)
	}

	fmt.Printf("# Latest %s deployment (%s)\n\n", dep.Env, dep.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	fmt.Print(string(data))
	return nil
}

func listDeployments(auditDir string) error {
	deployments, err := audit.ListDeployments(auditDir)
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		printInfo("No deployments found. Deploy first, then inspect.")
		return nil
	}

	fmt.Printf("%-24s  %-14s  %s\n", "TIMESTAMP", "ENVIRONMENT", "PATH")
	fmt.Printf("%-24s  %-14s  %s\n", strings.Repeat("-", 24), strings.Repeat("-", 14), strings.Repeat("-", 40))

	for _, d := range deployments {
		fmt.Printf("%-24s  %-14s  %s\n",
			d.Timestamp.Format("2006-01-02 15:04:05 UTC"),
			d.Env,
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
			fmt.Printf("# %s deployment (%s)\n\n", d.Env, d.Timestamp.Format("2006-01-02 15:04:05 UTC"))
			fmt.Print(string(data))
			return nil
		}
	}

	return hintErr(
		fmt.Errorf("no deployment matching prefix %q found", prefix),
		"Run `dargstack inspect --list` to see all available deployments.",
	)
}

func showDiff(auditDir string, latest *audit.Deployment, latestData []byte) error {
	// Build current compose for comparison
	var currentData []byte
	var err error

	if latest.Env == "production" {
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

	fmt.Printf("# Diff: last deployed vs current (%s)\n\n", latest.Env)

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
			printSuccess("No changes — current compose matches last deployed version")
			return nil
		}
		printWarning("diff not available; showing full current compose:")
		fmt.Print(string(currentData))
		return nil
	}

	if len(out) == 0 {
		printSuccess("No changes — current compose matches last deployed version")
		return nil
	}

	fmt.Print(string(out))
	return nil
}

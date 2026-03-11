package docker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Executor runs docker CLI commands, handling sudo as needed.
type Executor struct {
	useSudo bool
	binary  string
}

// NewExecutor creates an Executor that auto-detects sudo requirement
// or uses the configured preference.
func NewExecutor(sudoMode string) (*Executor, error) {
	binary, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("docker not found on PATH\n\n  Install Docker: https://docs.docker.com/get-docker/")
	}

	e := &Executor{binary: binary}

	switch sudoMode {
	case "always":
		e.useSudo = true
	case "never":
		e.useSudo = false
	default:
		e.useSudo = needsSudo(binary)
	}

	// Pre-validate sudo credentials so later commands (e.g. RunWithStdin
	// where stdin is piped) don't fail trying to prompt for a password.
	if e.useSudo {
		if err := prewarmSudo(); err != nil {
			return nil, fmt.Errorf("docker requires elevated privileges but sudo authentication failed: %w\n\n  Either add your user to the docker group (`sudo usermod -aG docker $USER`, then log out and back in)\n  or ensure sudo is configured correctly.", err) //nolint:staticcheck // intentional multi-line user hint
		}
	}

	return e, nil
}

func needsSudo(binary string) bool {
	cmd := exec.Command(binary, "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		// Only use sudo when it actually fixes the problem (permission denied).
		// If the daemon is down or Docker is broken, sudo won't help and we
		// should let the underlying error surface via the normal command path.
		sudoCmd := exec.Command("sudo", "-n", binary, "info")
		sudoCmd.Stdout = nil
		sudoCmd.Stderr = nil
		return sudoCmd.Run() == nil
	}
	return false
}

// prewarmSudo prompts the user for sudo credentials (if not already cached)
// by running `sudo -v` with the terminal attached. This caches credentials
// so that subsequent sudo invocations (even with stdin piped) succeed.
func prewarmSudo() error {
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Run executes a docker CLI command and returns stdout.
func (e *Executor) Run(args ...string) (string, error) {
	var cmd *exec.Cmd
	if e.useSudo {
		fullArgs := append([]string{e.binary}, args...)
		cmd = exec.Command("sudo", fullArgs...)
	} else {
		cmd = exec.Command(e.binary, args...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker %s: %s\n%s", strings.Join(args, " "), err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// RunPassthrough executes a docker command with stdout/stderr connected to the terminal.
func (e *Executor) RunPassthrough(args ...string) error {
	var cmd *exec.Cmd
	if e.useSudo {
		fullArgs := append([]string{e.binary}, args...)
		cmd = exec.Command("sudo", fullArgs...)
	} else {
		cmd = exec.Command(e.binary, args...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// RunWithStdin executes a docker command passing data via stdin.
func (e *Executor) RunWithStdin(input []byte, args ...string) error {
	var cmd *exec.Cmd
	if e.useSudo {
		fullArgs := append([]string{e.binary}, args...)
		cmd = exec.Command("sudo", fullArgs...)
	} else {
		cmd = exec.Command(e.binary, args...)
	}

	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// NeedsSudo reports whether the executor uses sudo for commands.
func (e *Executor) NeedsSudo() bool {
	return e.useSudo
}

// Ping checks whether the Docker daemon is reachable via the CLI.
func (e *Executor) Ping() error {
	_, err := e.Run("info", "--format", "{{.ID}}")
	return err
}

// SwarmActive checks whether Docker Swarm is active via the CLI.
func (e *Executor) SwarmActive() (bool, error) {
	out, err := e.Run("info", "--format", "{{.Swarm.LocalNodeState}}")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "active", nil
}

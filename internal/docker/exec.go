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
	useSudo    bool
	binary     string
	composeEnv map[string]string // extra vars forwarded explicitly in sudo RunWithStdin
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

		// First try non-interactive (fast, no prompt) to see if credentials are cached.
		sudoNI := exec.Command("sudo", "-n", binary, "info")
		sudoNI.Stdout = nil
		sudoNI.Stderr = nil
		if sudoNI.Run() == nil {
			return true
		}
		// Credentials may have expired. Fall back to interactive sudo so the
		// user can authenticate once rather than getting a silent permission error.
		fmt.Fprintln(os.Stderr, "sudo: Docker requires elevated privileges on this system. Please authenticate to continue.")
		sudoI := exec.Command("sudo", binary, "info")
		sudoI.Stdin = os.Stdin
		sudoI.Stdout = nil
		sudoI.Stderr = os.Stderr
		return sudoI.Run() == nil
	}
	return false
}

// prewarmSudo prompts the user for sudo credentials when they are not cached.
// A hint explaining why sudo is needed is printed only when a password prompt
// is actually about to appear. If credentials are already valid, returns nil
// immediately without any output.
func prewarmSudo() error {
	// Fast non-interactive check: if credentials are cached, skip the prompt.
	ni := exec.Command("sudo", "-n", "-v")
	ni.Stdout = nil
	ni.Stderr = nil
	if ni.Run() == nil {
		return nil
	}
	// Credentials not cached — a password prompt is about to appear.
	fmt.Fprintln(os.Stderr, "sudo: Docker requires elevated privileges on this system. Please authenticate to continue.")
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// refreshSudoIfNeeded validates sudo credentials and re-prompts if expired.
// The hint and password prompt are shown only when credentials are not cached.
func refreshSudoIfNeeded() error {
	return prewarmSudo()
}

// Run executes a docker CLI command and returns stdout.
func (e *Executor) Run(args ...string) (string, error) {
	if e.useSudo {
		if err := refreshSudoIfNeeded(); err != nil {
			return "", fmt.Errorf("sudo authentication failed: %w", err)
		}
	}
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
// When sudo is required, any compose environment variables registered via
// SetComposeEnv are forwarded explicitly using `sudo env KEY=VAL…` so that
// Docker Compose variable substitution works without inheriting the full user
// environment (which would break rootless Docker socket paths etc.).
func (e *Executor) RunWithStdin(input []byte, args ...string) error {
	if e.useSudo {
		if err := refreshSudoIfNeeded(); err != nil {
			return fmt.Errorf("sudo authentication failed: %w", err)
		}
	}
	var cmd *exec.Cmd
	if e.useSudo {
		// Build: sudo env KEY=VAL ... docker args
		// This forwards only the compose variable-substitution env vars, not the
		// full user environment, so Docker socket routing is unaffected.
		envArgs := []string{"env"}
		for k, v := range e.composeEnv {
			envArgs = append(envArgs, fmt.Sprintf("%s=%s", k, v))
		}
		fullArgs := append(append(envArgs, e.binary), args...)
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

// SetComposeEnv stores environment variables to be forwarded explicitly when
// running `docker stack deploy` via sudo. This avoids inheriting the full user
// environment (which would break rootless Docker socket routing).
func (e *Executor) SetComposeEnv(env map[string]string) {
	e.composeEnv = env
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

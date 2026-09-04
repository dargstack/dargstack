// Package sudo centralizes privilege escalation through the sudo binary.
package sudo

import (
	"os"
	"os/exec"
	"runtime"

	"charm.land/lipgloss/v2"

	"github.com/dargstack/dargstack/v4/internal/logger"
)

// Available reports whether a command could be elevated with sudo.
// It is false when the process already runs as root, when the platform has no sudo, and when no sudo binary is on PATH.
func Available() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	if os.Geteuid() == 0 {
		return false
	}
	_, err := exec.LookPath("sudo")
	return err == nil
}

// Notify prints the one-line hint that explains why a sudo password prompt is about to appear.
// The reason should be a complete sentence naming what needs the elevated privileges.
func Notify(reason string) {
	_, _ = lipgloss.Fprintln(os.Stderr, logger.StyleWarn.Render("sudo: "+reason+" Please authenticate to continue."))
}

// Prewarm makes sure sudo credentials are cached so that later sudo commands never have to prompt themselves, which fails whenever their stdin is piped.
// The reason is printed only when a password prompt is actually about to appear, so runs with cached credentials stay silent.
// If credentials are already valid, it returns nil immediately without any output.
func Prewarm(reason string) error {
	// Fast non-interactive check: if credentials are cached, skip the prompt.
	ni := exec.Command("sudo", "-n", "-v")
	ni.Stdout = nil
	ni.Stderr = nil
	if ni.Run() == nil {
		return nil
	}
	// Credentials not cached; a password prompt is about to appear.
	Notify(reason)
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

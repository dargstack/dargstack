//go:build !windows

package giturl

import (
	"os"
	"os/exec"
	"syscall"
)

// SSHAgentAvailable returns true if an SSH agent is running and accessible.
func SSHAgentAvailable() bool {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return false
	}

	info, err := os.Stat(sock)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSocket == 0 {
		return false
	}

	cmd := exec.Command("ssh-add", "-l")
	cmd.Env = append(os.Environ(), "SSH_AUTH_SOCK="+sock)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	return cmd.Run() == nil
}

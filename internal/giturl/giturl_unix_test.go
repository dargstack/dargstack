//go:build !windows

package giturl

import "testing"

func TestSSHAgentAvailable_NoEnvVar(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	if SSHAgentAvailable() {
		t.Error("SSHAgentAvailable() should be false when SSH_AUTH_SOCK is not set")
	}
}

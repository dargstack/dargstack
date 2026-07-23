//go:build windows

package giturl

// SSHAgentAvailable always returns false on Windows as SSH_AUTH_SOCK
// unix socket mechanism is not available.
func SSHAgentAvailable() bool {
	return false
}

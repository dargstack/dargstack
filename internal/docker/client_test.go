package docker

import "testing"

func TestContextHostDefersToDockerHost(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/explicit.sock")
	if got := contextHost(); got != "" {
		t.Fatalf("contextHost() = %q, want \"\" so client.FromEnv keeps DOCKER_HOST", got)
	}
}

package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakePruneDocker extends the fake of fakeSwarmDocker with `node ls`, which prints DARGSTACK_TEST_NODES or fails when it is unset.
func fakePruneDocker(t *testing.T, logPath string) *Executor {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "docker")
	const body = `#!/bin/sh
echo "$*" >> "$DARGSTACK_TEST_LOG_FILE"
if [ "$1" = "node" ]; then
	[ -n "$DARGSTACK_TEST_NODES" ] || exit 1
	printf '%s' "$DARGSTACK_TEST_NODES"
elif [ "$1" = "service" ] && [ "$2" = "create" ]; then
	echo "svcid"
elif [ "$1" = "service" ] && [ "$2" = "ps" ]; then
	printf '%s' "$DARGSTACK_TEST_PS"
fi
exit 0
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DARGSTACK_TEST_LOG_FILE", logPath)
	pullPollInterval = time.Millisecond
	pullNoTaskGrace = time.Millisecond
	t.Cleanup(func() {
		pullPollInterval = 2 * time.Second
		pullNoTaskGrace = 15 * time.Second
	})
	return &Executor{binary: script}
}

const testNodes = "self1\ttrue\tReady\tActive\nn2\tfalse\tReady\tActive\nn3\tfalse\tReady\tDrain"

func readLog(t *testing.T, p string) string {
	t.Helper()
	b, _ := os.ReadFile(p)
	return string(b)
}

func TestPruneOtherNodesComplete(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "log")
	exec := fakePruneDocker(t, logPath)
	t.Setenv("DARGSTACK_TEST_NODES", testNodes)
	t.Setenv("DARGSTACK_TEST_PS", "host2\tComplete 2 seconds ago\t")

	pruned, failures, err := PruneOtherNodes(exec, "stk", time.Second)
	if err != nil || len(failures) != 0 || len(pruned) != 1 || pruned[0] != "host2" {
		t.Fatalf("pruned = %v, failures = %v, err = %v", pruned, failures, err)
	}
	log := readLog(t, logPath)
	for _, want := range []string{
		"--mode global-job", "--constraint node.id!=self1",
		"--mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock",
		"--entrypoint sh", "docker:cli -c docker container prune -f && docker image prune -af",
		"service rm stk_prune",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
}

func TestPruneOtherNodesFailedTask(t *testing.T) {
	exec := fakePruneDocker(t, filepath.Join(t.TempDir(), "log"))
	t.Setenv("DARGSTACK_TEST_NODES", testNodes)
	t.Setenv("DARGSTACK_TEST_PS", "host2\tFailed 1 second ago\texit 1")

	pruned, failures, err := PruneOtherNodes(exec, "stk", time.Second)
	if err != nil || len(pruned) != 0 || len(failures) != 1 || !strings.Contains(failures[0], "host2") {
		t.Fatalf("pruned = %v, failures = %v, err = %v", pruned, failures, err)
	}
}

func TestPruneOtherNodesNoSwarm(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "log")
	exec := fakePruneDocker(t, logPath)

	pruned, failures, err := PruneOtherNodes(exec, "stk", time.Second)
	if err != nil || len(pruned) != 0 || len(failures) != 0 {
		t.Fatalf("pruned = %v, failures = %v, err = %v", pruned, failures, err)
	}
	if strings.Contains(readLog(t, logPath), "service create") {
		t.Error("job created")
	}
}

func TestPruneOtherNodesOnlySelf(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "log")
	exec := fakePruneDocker(t, logPath)
	t.Setenv("DARGSTACK_TEST_NODES", "self1\ttrue\tReady\tActive")

	pruned, failures, err := PruneOtherNodes(exec, "stk", time.Second)
	if err != nil || len(pruned) != 0 || len(failures) != 0 {
		t.Fatalf("pruned = %v, failures = %v, err = %v", pruned, failures, err)
	}
	if strings.Contains(readLog(t, logPath), "service create") {
		t.Error("job created")
	}
}

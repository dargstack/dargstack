package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeSwarmDocker logs every call to DARGSTACK_TEST_LOG_FILE.
// `service ps <job>` prints the env var DARGSTACK_TEST_PS_<job>, with "-" in the job name replaced by "_".
func fakeSwarmDocker(t *testing.T, logPath string) *Executor {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "docker")
	const body = `#!/bin/sh
echo "$*" >> "$DARGSTACK_TEST_LOG_FILE"
if [ "$1" = "service" ] && [ "$2" = "create" ]; then
	echo "svcid"
elif [ "$1" = "service" ] && [ "$2" = "ps" ]; then
	var="DARGSTACK_TEST_PS_$(echo "$3" | tr '-' '_')"
	eval "printf '%s' \"\$$var\""
fi
exit 0
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DARGSTACK_TEST_LOG_FILE", logPath)
	pullPollInterval = time.Millisecond
	t.Cleanup(func() {
		pullPollInterval = 2 * time.Second
		pullNoTaskGrace = 15 * time.Second
	})
	return &Executor{binary: script}
}

func TestPullOnNodesRejectedTaskFails(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "log")
	exec := fakeSwarmDocker(t, logPath)
	t.Setenv("DARGSTACK_TEST_PS_stk_pull_0", "node1\tComplete 3 seconds ago\t\nnode2\tRejected 2 seconds ago\tNo such image: x")

	targets := []PullTarget{{Image: "x", Constraints: []string{"node.role==worker"}, Services: []string{"a", "b"}}}
	failures, err := PullOnNodes(exec, "stk", targets, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 1 || !strings.Contains(failures[0], "node2") || !strings.Contains(failures[0], "No such image: x") {
		t.Fatalf("failures = %v", failures)
	}

	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(b)
	for _, want := range []string{"--mode global-job", "--with-registry-auth", "--entrypoint true", "--constraint node.role==worker", "service rm stk_pull-0"} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
}

func TestPullOnNodesFailedTaskCountsAsPulled(t *testing.T) {
	exec := fakeSwarmDocker(t, filepath.Join(t.TempDir(), "log"))
	t.Setenv("DARGSTACK_TEST_PS_stk_pull_0", "node1\tFailed 1 second ago\tstarting container failed")

	failures, err := PullOnNodes(exec, "stk", []PullTarget{{Image: "x", Services: []string{"a"}}}, time.Second)
	if err != nil || len(failures) != 0 {
		t.Fatalf("failures = %v, err = %v", failures, err)
	}
}

func TestPullOnNodesSkipsResolvingDigests(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "log")
	exec := fakeSwarmDocker(t, logPath)
	t.Setenv("DARGSTACK_TEST_PS_stk_pull_0", "node1\tComplete 1 second ago\t")
	t.Setenv("DARGSTACK_TEST_PS_stk_pull_1", "node1\tComplete 1 second ago\t")

	targets := []PullTarget{
		{Image: "x:1@sha256:abc", Services: []string{"a"}},
		{Image: "y:1", Services: []string{"b"}},
	}
	if failures, err := PullOnNodes(exec, "stk", targets, time.Second); err != nil || len(failures) != 0 {
		t.Fatalf("failures = %v, err = %v", failures, err)
	}

	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	var creates int
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "service create") {
			continue
		}
		creates++
		if strings.HasSuffix(line, "x:1@sha256:abc") != strings.Contains(line, "--no-resolve-image") {
			t.Errorf("--no-resolve-image must be set exactly for the digest-pinned image: %s", line)
		}
	}
	if creates != 2 {
		t.Errorf("created %d jobs, want 2:\n%s", creates, b)
	}
	if !strings.Contains(string(b), "service rm stk_pull-0 stk_pull-1") {
		t.Errorf("jobs not removed in one call:\n%s", b)
	}
}

func TestPullOnNodesNoTasksFails(t *testing.T) {
	exec := fakeSwarmDocker(t, filepath.Join(t.TempDir(), "log"))
	pullNoTaskGrace = 0

	failures, err := PullOnNodes(exec, "stk", []PullTarget{{Image: "x", Services: []string{"a"}}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 1 || !strings.Contains(failures[0], "no node satisfies") {
		t.Fatalf("failures = %v", failures)
	}
}

func TestPullJobNameTruncatesStack(t *testing.T) {
	if got := pullJobName(strings.Repeat("s", 80), 12); len(got) != maxServiceNameLen || !strings.HasSuffix(got, "_pull-12") {
		t.Errorf("name = %q", got)
	}
}

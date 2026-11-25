package docker

import (
	"os"
	osexec "os/exec"
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

	pruned, failures, err := PruneOtherNodes(exec, "stk", []string{"a", "b"}, time.Second)
	if err != nil || len(failures) != 0 || len(pruned) != 1 || pruned[0] != "host2" {
		t.Fatalf("pruned = %v, failures = %v, err = %v", pruned, failures, err)
	}
	log := readLog(t, logPath)
	for _, want := range []string{
		"--mode global-job", "--constraint node.id!=self1",
		"--mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock",
		"--env DARGSTACK_KEEP=a b", "--entrypoint sh", "docker:cli -c " + pruneScript,
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

	pruned, failures, err := PruneOtherNodes(exec, "stk", []string{"a", "b"}, time.Second)
	if err != nil || len(pruned) != 0 || len(failures) != 1 || !strings.Contains(failures[0], "host2") {
		t.Fatalf("pruned = %v, failures = %v, err = %v", pruned, failures, err)
	}
}

func TestPruneOtherNodesNoSwarm(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "log")
	exec := fakePruneDocker(t, logPath)

	pruned, failures, err := PruneOtherNodes(exec, "stk", []string{"a", "b"}, time.Second)
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

	pruned, failures, err := PruneOtherNodes(exec, "stk", []string{"a", "b"}, time.Second)
	if err != nil || len(pruned) != 0 || len(failures) != 0 {
		t.Fatalf("pruned = %v, failures = %v, err = %v", pruned, failures, err)
	}
	if strings.Contains(readLog(t, logPath), "service create") {
		t.Error("job created")
	}
}

// fakeKeepDocker logs every call and answers the queries of the prune algorithm.
func writeFakeKeepDocker(t *testing.T, logPath string) string {
	t.Helper()
	dir := t.TempDir()
	const body = `#!/bin/sh
echo "$*" >> "$DARGSTACK_TEST_LOG_FILE"
case "$1 $2" in
"image inspect")
	[ "$5" = "keepref" ] && echo sha256:keep
	;;
"ps -aq") echo c1 ;;
"inspect --format") echo sha256:used ;;
"image ls") printf 'sha256:keep\nsha256:used\nsha256:old1\nsha256:old2\nsha256:old1\n' ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DARGSTACK_TEST_LOG_FILE", logPath)
	return dir
}

func rmLines(log string) []string {
	var out []string
	for _, l := range strings.Split(log, "\n") {
		if strings.HasPrefix(l, "image rm -f ") {
			out = append(out, l)
		}
	}
	return out
}

func wantOldRemoved(t *testing.T, log string) {
	t.Helper()
	got := strings.Join(rmLines(log), "|")
	if got != "image rm -f sha256:old1|image rm -f sha256:old2" {
		t.Errorf("rm calls = %q\n%s", got, log)
	}
}

func TestPruneImagesKeeping(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "log")
	dir := writeFakeKeepDocker(t, logPath)

	removed, err := PruneImagesKeeping(&Executor{binary: filepath.Join(dir, "docker")}, []string{"keepref", "missing"})
	if err != nil || removed != 2 {
		t.Fatalf("removed = %d, err = %v", removed, err)
	}
	wantOldRemoved(t, readLog(t, logPath))
}

func TestPruneScript(t *testing.T) {
	sh, err := osexec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	logPath := filepath.Join(t.TempDir(), "log")
	dir := writeFakeKeepDocker(t, logPath)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DARGSTACK_KEEP", "keepref missing")

	if out, err := osexec.Command(sh, "-c", pruneScript).CombinedOutput(); err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	wantOldRemoved(t, readLog(t, logPath))
}

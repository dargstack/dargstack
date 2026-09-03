package docker

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fakeDockerBinary writes a shell script standing in for the real docker CLI.
// `docker image inspect` succeeds only for images listed (comma-separated) in the DARGSTACK_TEST_LOCAL_IMAGES env var, simulating which images are already present locally.
// `docker manifest inspect` appends the probed image to logPath and fails for images listed in DARGSTACK_TEST_FAIL_IMAGES, simulating registry accessibility.
func fakeDockerBinary(t *testing.T, logPath string) string {
	t.Helper()

	dir := t.TempDir()
	script := filepath.Join(dir, "docker")

	const body = `#!/bin/sh
if [ "$1" = "image" ] && [ "$2" = "inspect" ]; then
	img="$5"
	case ",$DARGSTACK_TEST_LOCAL_IMAGES," in
		*",$img,"*) echo "sha256:fake"; exit 0 ;;
		*) exit 1 ;;
	esac
elif [ "$1" = "manifest" ] && [ "$2" = "inspect" ]; then
	img="$3"
	echo "$img" >> "$DARGSTACK_TEST_LOG_FILE"
	case ",$DARGSTACK_TEST_FAIL_IMAGES," in
		*",$img,"*) exit 1 ;;
		*) exit 0 ;;
	esac
fi
exit 1
`

	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake docker binary: %v", err)
	}
	t.Setenv("DARGSTACK_TEST_LOG_FILE", logPath)
	return script
}

func TestCheckImagesAccessibleSkipsLocalImages(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "manifest-checks.log")
	binary := fakeDockerBinary(t, logPath)
	t.Setenv("DARGSTACK_TEST_LOCAL_IMAGES", "local-image,another-local-image")
	t.Setenv("DARGSTACK_TEST_FAIL_IMAGES", "unreachable-image")

	exec := &Executor{binary: binary}
	images := []string{"local-image", "remote-image", "another-local-image", "unreachable-image"}

	failed := CheckImagesAccessible(exec, images)

	if len(failed) != 1 {
		t.Fatalf("failed = %v, want exactly one entry for unreachable-image", failed)
	}
	if _, ok := failed["unreachable-image"]; !ok {
		t.Errorf("expected unreachable-image to be reported as failed, got %v", failed)
	}

	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read manifest check log: %v", err)
	}
	var probed []string
	for _, line := range strings.Split(strings.TrimSpace(string(logged)), "\n") {
		if line != "" {
			probed = append(probed, line)
		}
	}
	sort.Strings(probed)

	want := []string{"remote-image", "unreachable-image"}
	if len(probed) != len(want) {
		t.Fatalf("manifest inspect probed %v, want %v (local images must not trigger a registry call)", probed, want)
	}
	for i, img := range want {
		if probed[i] != img {
			t.Errorf("manifest inspect probed %v, want %v", probed, want)
			break
		}
	}
}

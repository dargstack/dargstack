package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestNeedsSudo covers the sudo auto-detection decision for every failure mode of `docker info`.
// Fake docker and sudo binaries stand in for the real ones, so no daemon, credentials, or interactive prompt is involved.
func TestNeedsSudo(t *testing.T) {
	cases := []struct {
		name           string
		dockerStderr   string
		dockerExit     int
		sudoNExit      int
		sudoExit       int
		wantSudo       bool
		wantSudoCalled bool
	}{
		{
			name:       "daemon reachable",
			dockerExit: 0,
			wantSudo:   false,
		},
		{
			name:         "daemon down does not escalate to sudo",
			dockerStderr: "failed to connect to the docker API at unix:///var/run/docker.sock; check if the path is correct and if the daemon is running: dial unix /var/run/docker.sock: connect: no such file or directory",
			dockerExit:   1,
			wantSudo:     false,
		},
		{
			name:         "stale socket after machine sleep does not escalate to sudo",
			dockerStderr: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?",
			dockerExit:   1,
			wantSudo:     false,
		},
		{
			name:           "permission denied with cached sudo credentials escalates",
			dockerStderr:   "permission denied while trying to connect to the docker API at unix:///var/run/docker.sock",
			dockerExit:     1,
			sudoNExit:      0,
			wantSudo:       true,
			wantSudoCalled: true,
		},
		{
			name:           "permission denied without cached credentials prompts and escalates",
			dockerStderr:   "Got permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock: dial unix /var/run/docker.sock: connect: permission denied",
			dockerExit:     1,
			sudoNExit:      1,
			sudoExit:       0,
			wantSudo:       true,
			wantSudoCalled: true,
		},
		{
			name:           "permission denied but sudo cannot reach the daemon either",
			dockerStderr:   "permission denied while trying to connect to the docker API at unix:///var/run/docker.sock",
			dockerExit:     1,
			sudoNExit:      1,
			sudoExit:       1,
			wantSudo:       false,
			wantSudoCalled: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			binDir := filepath.Join(t.TempDir(), "bin")
			if err := os.MkdirAll(binDir, 0o755); err != nil {
				t.Fatal(err)
			}

			docker := filepath.Join(binDir, "docker")
			var dockerScript string
			if tc.dockerStderr != "" {
				dockerScript += fmt.Sprintf("echo %q >&2\n", tc.dockerStderr)
			}
			dockerScript = "#!/bin/sh\n" + dockerScript + fmt.Sprintf("exit %d\n", tc.dockerExit)
			if err := os.WriteFile(docker, []byte(dockerScript), 0o755); err != nil {
				t.Fatal(err)
			}

			sudoLog := filepath.Join(t.TempDir(), "sudo.log")
			sudo := filepath.Join(binDir, "sudo")
			sudoScript := fmt.Sprintf("#!/bin/sh\necho \"$*\" >> %q\nif [ \"$1\" = \"-n\" ]; then\n\texit \"$DARGSTACK_TEST_SUDO_N_EXIT\"\nfi\nexit \"$DARGSTACK_TEST_SUDO_EXIT\"\n", sudoLog)
			if err := os.WriteFile(sudo, []byte(sudoScript), 0o755); err != nil {
				t.Fatal(err)
			}

			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("DARGSTACK_TEST_SUDO_N_EXIT", strconv.Itoa(tc.sudoNExit))
			t.Setenv("DARGSTACK_TEST_SUDO_EXIT", strconv.Itoa(tc.sudoExit))

			if got := needsSudo(docker); got != tc.wantSudo {
				t.Fatalf("needsSudo() = %v, want %v", got, tc.wantSudo)
			}

			logData, err := os.ReadFile(sudoLog)
			switch {
			case err == nil && tc.wantSudoCalled:
			case err == nil:
				t.Fatalf("sudo was invoked, want it to never be invoked:\n%s", logData)
			case err != nil && tc.wantSudoCalled:
				t.Fatalf("sudo was never invoked, want it to be invoked: %v", err)
			}
		})
	}
}

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/dargstack/dargstack/v4/internal/docker"
)

// fakeDockerOnPath puts a stub `docker` binary on PATH for the duration of the
// test. `docker image inspect` always fails (no image is "local"); `docker
// manifest inspect <image>` fails only for images listed (comma-separated) in
// DARGSTACK_TEST_FAIL_IMAGES, simulating registry accessibility.
func fakeDockerOnPath(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	script := filepath.Join(dir, "docker")
	const body = `#!/bin/sh
if [ "$1" = "image" ] && [ "$2" = "inspect" ]; then
	exit 1
elif [ "$1" = "manifest" ] && [ "$2" = "inspect" ]; then
	img="$3"
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
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestDeployPreDeployChecks_NilExecutorSkipsSilently(t *testing.T) {
	composeYAML := `services:
  app:
    image: example/app:latest
`
	if _, err := deployPreDeployChecks(nil, []byte(composeYAML), true); err != nil {
		t.Fatalf("expected nil executor to skip the check rather than fail, got: %v", err)
	}
}

func TestDeployPreDeployChecks_DryRunRunsRealCheck(t *testing.T) {
	fakeDockerOnPath(t)
	t.Setenv("DARGSTACK_TEST_FAIL_IMAGES", "example/unreachable:latest")

	executor, err := docker.NewExecutor("never")
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}

	composeYAML := `services:
  app:
    image: example/unreachable:latest
`

	if _, err := deployPreDeployChecks(executor, []byte(composeYAML), true); err == nil {
		t.Fatal("expected dry-run to surface the unreachable image as an error, got nil")
	}
}

func TestDeployValidateResources_DryRunActuallyValidates(t *testing.T) {
	dir := t.TempDir()

	// References a secret file that doesn't exist on disk, which validateSecrets
	// flags as an error in development mode.
	composeYAML := `secrets:
  api-key:
    file: ` + dir + `/api-key.secret
`

	origStackDir, origEnv := stackDir, env
	stackDir, env = dir, ""
	defer func() { stackDir, env = origStackDir, origEnv }()

	if err := deployValidateResources([]byte(composeYAML), nil, true); err == nil {
		t.Fatal("expected dry-run to surface the missing secret file as a validation error, got nil")
	}
}

func TestFilterVolumesByCompose(t *testing.T) {
	composeYAML := `services:
  postgres:
    image: postgres:16
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata: {}
`
	allVolumes := []string{"mystack_pgdata", "mystack_redisdata"}
	got := filterVolumesByCompose(allVolumes, "mystack", []byte(composeYAML))
	if len(got) != 1 || got[0] != "mystack_pgdata" {
		t.Fatalf("expected [mystack_pgdata], got %v", got)
	}
}

func TestExtractBuildServices(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []string
	}{
		{
			name:     "no services",
			yaml:     "version: '3'",
			expected: nil,
		},
		{
			name: "services without build label",
			yaml: `
services:
  web:
    image: nginx
  api:
    image: node
`,
			expected: nil,
		},
		{
			name: "single service with build label",
			yaml: `
services:
  web:
    image: nginx
    deploy:
      labels:
        dargstack.development.build: "./"
`,
			expected: []string{"web"},
		},
		{
			name: "multiple services mixed",
			yaml: `
services:
  web:
    image: nginx
    deploy:
      labels:
        dargstack.development.build: "./"
  api:
    image: node
  worker:
    image: python
    deploy:
      labels:
        dargstack.development.build: "../docker"
`,
			expected: []string{"web", "worker"},
		},
		{
			name: "labels as list format",
			yaml: `
services:
  app:
    image: ruby
    deploy:
      labels:
        - dargstack.development.build=./app
`,
			expected: []string{"app"},
		},
		{
			name:     "invalid yaml",
			yaml:     "{{invalid",
			expected: nil,
		},
		{
			name:     "empty yaml",
			yaml:     "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBuildServices([]byte(tt.yaml))
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, result)
					return
				}
			}
		})
	}
}

func TestCountComposeServices(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected int
	}{
		{
			name:     "no services key",
			yaml:     "version: '3'",
			expected: 0,
		},
		{
			name: "single service",
			yaml: `
services:
  web:
    image: nginx
`,
			expected: 1,
		},
		{
			name: "multiple services",
			yaml: `
services:
  web:
    image: nginx
  api:
    image: node
  worker:
    image: python
`,
			expected: 3,
		},
		{
			name:     "empty services",
			yaml:     "services: {}",
			expected: 0,
		},
		{
			name:     "invalid yaml",
			yaml:     "{{invalid",
			expected: 0,
		},
		{
			name:     "empty yaml",
			yaml:     "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countComposeServices([]byte(tt.yaml))
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestInjectBuildContextMapLabels(t *testing.T) {
	yamlIn := `
services:
  web:
    image: nginx
    deploy:
      labels:
        dargstack.development.git: "git@github.com:organization/repository.git"
`
	result, err := injectBuildContext([]byte(yamlIn), "web", "/path/to/repository")
	if err != nil {
		t.Fatalf("injectBuildContext failed: %v", err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	svcMap := doc["services"].(map[string]interface{})
	svc := svcMap["web"].(map[string]interface{})
	deploy := svc["deploy"].(map[string]interface{})
	labels := deploy["labels"].(map[string]interface{})
	build := labels["dargstack.development.build"].(string)

	if build != "/path/to/repository" {
		t.Errorf("expected /path/to/repository, got %q", build)
	}
}

func TestInjectBuildContextListLabels(t *testing.T) {
	yamlIn := `
services:
  web:
    image: nginx
    deploy:
      labels:
        - dargstack.development.git=git@github.com:organization/repository.git
        - other.label=value
`
	result, err := injectBuildContext([]byte(yamlIn), "web", "/path/to/repository")
	if err != nil {
		t.Fatalf("injectBuildContext failed: %v", err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	svcMap := doc["services"].(map[string]interface{})
	svc := svcMap["web"].(map[string]interface{})
	deploy := svc["deploy"].(map[string]interface{})
	labels := deploy["labels"].([]interface{})

	found := false
	for _, item := range labels {
		s, ok := item.(string)
		if ok && s == "dargstack.development.build=/path/to/repository" {
			found = true
		}
	}
	if !found {
		t.Error("expected build label in list")
	}
}

func TestInjectBuildContextNoOpIfBuildExists(t *testing.T) {
	yamlIn := `
services:
  web:
    image: nginx
    deploy:
      labels:
        dargstack.development.git: "git@github.com:organization/repository.git"
        dargstack.development.build: "./custom"
`
	result, err := injectBuildContext([]byte(yamlIn), "web", "/path/to/repository")
	if err != nil {
		t.Fatalf("injectBuildContext failed: %v", err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	svcMap := doc["services"].(map[string]interface{})
	svc := svcMap["web"].(map[string]interface{})
	deploy := svc["deploy"].(map[string]interface{})
	labels := deploy["labels"].(map[string]interface{})
	build := labels["dargstack.development.build"].(string)

	if build != "./custom" {
		t.Errorf("expected ./custom (existing build should not be overridden), got %q", build)
	}
}

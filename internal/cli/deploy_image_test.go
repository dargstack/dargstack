package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dargstack/dargstack/v4/internal/config"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
}

func setupGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgSign", "false")

	f, err := os.Create(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
}

func TestLatestGitTag(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	runGit(t, dir, "tag", "v0.1.0")

	// Second commit so v1.0.0 is reachable after v0.1.0
	f, err := os.Create(filepath.Join(dir, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "v1.0.0")

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	tag, err := latestGitTag("main")
	if err != nil {
		t.Fatalf("latestGitTag failed: %v", err)
	}
	if tag != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", tag)
	}
}

func TestLatestGitTagNoTags(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	_, err := latestGitTag("main")
	if err == nil {
		t.Fatal("expected error when no tags exist")
	}
}

func TestResolveDeployTagExplicitFlag(t *testing.T) {
	origDeployTag := deployTag
	origOffline := offline
	defer func() {
		deployTag = origDeployTag
		offline = origOffline
	}()

	deployTag = "v2.0.0"
	offline = false

	tag, err := resolveDeployTag()
	if err != nil {
		t.Fatalf("resolveDeployTag failed: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %s", tag)
	}
}

func TestResolveDeployTagPinnedConfig(t *testing.T) {
	origDeployTag := deployTag
	origOffline := offline
	origCfg := cfg
	defer func() {
		deployTag = origDeployTag
		offline = origOffline
		cfg = origCfg
	}()

	deployTag = ""
	offline = false
	cfg = &config.Config{
		Production: config.ProductionConfig{
			Tag:    "v3.0.0",
			Branch: "main",
		},
	}

	tag, err := resolveDeployTag()
	if err != nil {
		t.Fatalf("resolveDeployTag failed: %v", err)
	}
	if tag != "v3.0.0" {
		t.Errorf("expected v3.0.0, got %s", tag)
	}
}

func TestResolveDeployTagFromGit(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v1.2.3")

	origDeployTag := deployTag
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = ""
	offline = true
	cfg = &config.Config{
		Production: config.ProductionConfig{
			Tag:    "latest",
			Branch: "main",
		},
	}
	stackDir = dir

	tag, err := resolveDeployTag()
	if err != nil {
		t.Fatalf("resolveDeployTag failed: %v", err)
	}
	if tag != "v1.2.3" {
		t.Errorf("expected v1.2.3, got %s", tag)
	}
}

func TestGitFetchTagsSuccess(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	err := gitFetchTags()
	if err == nil {
		t.Fatal("expected error: no remote 'origin' configured")
	}
}

func TestGitFetchTagsNoRemote(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	err := gitFetchTags()
	if err == nil {
		t.Fatal("expected error when no origin remote exists")
	}
	if !strings.Contains(err.Error(), "origin") {
		t.Errorf("expected error mentioning origin, got: %v", err)
	}
}

func TestRepoNameFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "ssh format",
			url:      "git@github.com:organization/repository.git",
			expected: "repository",
		},
		{
			name:     "ssh format no .git",
			url:      "git@github.com:organization/repository",
			expected: "repository",
		},
		{
			name:     "https format",
			url:      "https://github.com/organization/repository.git",
			expected: "repository",
		},
		{
			name:     "https format no .git",
			url:      "https://github.com/organization/repository",
			expected: "repository",
		},
		{
			name:     "git protocol",
			url:      "git://github.com/organization/repository.git",
			expected: "repository",
		},
		{
			name:     "bitbucket ssh",
			url:      "git@bitbucket.org:team/project-repo.git",
			expected: "project-repo",
		},
		{
			name:     "self-hosted with port",
			url:      "git@192.168.1.1:22/path/to/repo.git",
			expected: "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repoNameFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("repoNameFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestExtractDargstackGitLabel(t *testing.T) {
	tests := []struct {
		name     string
		svc      map[string]interface{}
		expected string
	}{
		{
			name: "labels as map",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{
						"dargstack.development.git": "git@github.com:organization/repository.git",
					},
				},
			},
			expected: "git@github.com:organization/repository.git",
		},
		{
			name: "labels as list with key=value",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": []interface{}{
						"dargstack.development.git=git@github.com:organization/repository.git",
						"other.label=value",
					},
				},
			},
			expected: "git@github.com:organization/repository.git",
		},
		{
			name: "no deploy key",
			svc: map[string]interface{}{
				"image": "nginx",
			},
			expected: "",
		},
		{
			name: "no labels key",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"replicas": "3",
				},
			},
			expected: "",
		},
		{
			name: "labels map without git key",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{
						"dargstack.development.build": "./build",
					},
				},
			},
			expected: "",
		},
		{
			name: "labels list without git entry",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": []interface{}{
						"dargstack.development.build=./app",
					},
				},
			},
			expected: "",
		},
		{
			name: "both git and build labels",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{
						"dargstack.development.git":   "git@github.com:organization/repository.git",
						"dargstack.development.build": "./subdir",
					},
				},
			},
			expected: "git@github.com:organization/repository.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDargstackGitLabel(tt.svc)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractDargstackBuildContext(t *testing.T) {
	tests := []struct {
		name     string
		svc      map[string]interface{}
		expected string
	}{
		{
			name: "labels as map",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{
						"dargstack.development.build": "./build",
					},
				},
			},
			expected: "./build",
		},
		{
			name: "labels as list with key=value",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": []interface{}{
						"dargstack.development.build=../docker",
						"other.label=value",
					},
				},
			},
			expected: "../docker",
		},
		{
			name: "no deploy key",
			svc: map[string]interface{}{
				"image": "nginx",
			},
			expected: "",
		},
		{
			name: "no labels key",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"replicas": "3",
				},
			},
			expected: "",
		},
		{
			name: "labels map without build key",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{
						"other.label": "value",
					},
				},
			},
			expected: "",
		},
		{
			name: "labels list without build entry",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": []interface{}{
						"other.label=value",
					},
				},
			},
			expected: "",
		},
		{
			name: "labels list with non-string items",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": []interface{}{
						123,
						true,
					},
				},
			},
			expected: "",
		},
		{
			name: "deploy is not a map",
			svc: map[string]interface{}{
				"deploy": "string",
			},
			expected: "",
		},
		{
			name:     "empty service",
			svc:      map[string]interface{}{},
			expected: "",
		},
		{
			name: "labels as empty map",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": map[string]interface{}{},
				},
			},
			expected: "",
		},
		{
			name: "labels as empty list",
			svc: map[string]interface{}{
				"deploy": map[string]interface{}{
					"labels": []interface{}{},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDargstackBuildContext(tt.svc)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

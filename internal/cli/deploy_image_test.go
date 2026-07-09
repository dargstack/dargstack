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
	runGit(t, dir, "init")
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

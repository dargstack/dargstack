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

// setupGitRepoWithOrigin creates a working repo with a bare origin remote
// and fetches it so refs/remotes/origin/main exists locally.
func setupGitRepoWithOrigin(t *testing.T, workDir string) {
	t.Helper()
	bareDir := t.TempDir()
	runGit(t, bareDir, "init", "--bare")

	setupGitRepo(t, workDir)
	runGit(t, workDir, "remote", "add", "origin", bareDir)
	runGit(t, workDir, "push", "-u", "origin", "main")
	runGit(t, workDir, "fetch", "origin", "--tags")
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

func TestLatestGitTagPrefersOrigin(t *testing.T) {
	dir := t.TempDir()
	setupGitRepoWithOrigin(t, dir)

	// Create a tag that's only on the local repo at first.
	runGit(t, dir, "tag", "v1.0.0")

	// Push main, then add a newer tag and push it so origin has v2.0.0.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "v2.0.0")
	runGit(t, dir, "push", "origin", "main", "--tags")

	// Make local diverge with a newer tag that is *not* pushed, to verify we
	// prefer origin/<branch> over the local branch.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "local only")
	runGit(t, dir, "tag", "v3.0.0")
	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	tag, err := latestGitTag("main")
	if err != nil {
		t.Fatalf("latestGitTag failed: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("expected origin tag v2.0.0, got %s", tag)
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
		Environment: config.EnvironmentConfig{
			Production: config.ProdConfig{
				Tag:    "v3.0.0",
				Branch: "main",
			},
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
		Environment: config.EnvironmentConfig{
			Production: config.ProdConfig{
				Tag:    "",
				Branch: "main",
			},
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

func TestResolveDeployTagLiteralLatest(t *testing.T) {
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
		Environment: config.EnvironmentConfig{
			Production: config.ProdConfig{
				Tag:    "latest",
				Branch: "main",
			},
		},
	}

	tag, err := resolveDeployTag()
	if err != nil {
		t.Fatalf("resolveDeployTag failed: %v", err)
	}
	if tag != "latest" {
		t.Errorf("expected literal 'latest', got %s", tag)
	}
}

func TestGitFetchOriginErrorsWithoutRemote(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	err := gitFetchOrigin()
	if err == nil {
		t.Fatal("expected error: no remote 'origin' configured")
	}
	if !strings.Contains(err.Error(), "origin") {
		t.Errorf("expected error mentioning origin, got: %v", err)
	}
}

func TestGitWorkingTreeDirtyClean(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	dirty, err := gitWorkingTreeDirty(dir)
	if err != nil {
		t.Fatalf("gitWorkingTreeDirty failed: %v", err)
	}
	if dirty {
		t.Error("expected clean working tree")
	}
}

func TestGitWorkingTreeDirtyIgnoresUntracked(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirty, err := gitWorkingTreeDirty(dir)
	if err != nil {
		t.Fatalf("gitWorkingTreeDirty failed: %v", err)
	}
	if dirty {
		t.Error("expected untracked files to be ignored")
	}
}

func TestGitWorkingTreeDirtyDetectsModifiedTracked(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirty, err := gitWorkingTreeDirty(dir)
	if err != nil {
		t.Fatalf("gitWorkingTreeDirty failed: %v", err)
	}
	if !dirty {
		t.Error("expected modified tracked file to be reported as dirty")
	}
}

func TestCheckoutDeployTagChecksOutTag(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v1.0.0")

	// Second commit after the tag so main and v1.0.0 diverge.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")

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

	deployTag = "v1.0.0"
	offline = false
	cfg = &config.Config{}
	stackDir = dir

	tag, err := checkoutDeployTag()
	if err != nil {
		t.Fatalf("checkoutDeployTag failed: %v", err)
	}
	if tag != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", tag)
	}

	if _, err := os.Stat(filepath.Join(dir, "file.txt")); !os.IsNotExist(err) {
		t.Error("expected working tree to reflect v1.0.0, but file.txt from the later commit is present")
	}
}

func TestCheckoutDeployTagSkipsCheckoutWhenOffline(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v1.0.0")

	// Dirty the tree — should NOT error when offline
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

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

	deployTag = "v1.0.0"
	offline = true
	cfg = &config.Config{}
	stackDir = dir

	tag, err := checkoutDeployTag()
	if err != nil {
		t.Fatalf("checkoutDeployTag should not error when offline: %v", err)
	}
	if tag != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", tag)
	}
}

func TestCheckoutDeployTagRejectsDirtyTree(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v1.0.0")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

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

	deployTag = "v1.0.0"
	offline = false
	cfg = &config.Config{}
	stackDir = dir

	_, err := checkoutDeployTag()
	if err == nil {
		t.Fatal("expected error for dirty working tree")
	}
}

func TestParseMajorVersion(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		want    int
		wantErr bool
	}{
		{"prefixed minor", "v1.2.3", 1, false},
		{"unprefixed minor", "1.2.3", 1, false},
		{"prefixed zero", "v0.9.0", 0, false},
		{"multi-digit major", "v10.0.0", 10, false},
		{"unprefixed multi-digit", "10.0.0", 10, false},
		{"prefixed zero unprefixed", "0.1.0", 0, false},
		{"non-semver", "latest", 0, true},
		{"non-numeric", "vabc", 0, true},
		{"empty", "", 0, true},
		{"only prefix", "v", 0, true},
		{"major only", "v1", 1, false},
		{"major only unprefixed", "1", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMajorVersion(tt.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMajorVersion(%q) error = %v, wantErr %v", tt.tag, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseMajorVersion(%q) = %d, want %d", tt.tag, got, tt.want)
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

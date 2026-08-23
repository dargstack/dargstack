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

	tag, err := latestGitTag(1)
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

	_, err := latestGitTag(1)
	if err == nil {
		t.Fatal("expected error when no tags exist")
	}
}

func TestLatestGitTagFiltersByMajor(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	runGit(t, dir, "tag", "v1.0.0")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "v1.1.0")

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "third")
	runGit(t, dir, "tag", "v2.0.0")

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("3"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "fourth")
	runGit(t, dir, "tag", "v2.1.0")

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	tag, err := latestGitTag(1)
	if err != nil {
		t.Fatalf("latestGitTag failed: %v", err)
	}
	if tag != "v1.1.0" {
		t.Errorf("latestGitTag(1) = %q, want v1.1.0", tag)
	}

	tag, err = latestGitTag(2)
	if err != nil {
		t.Fatalf("latestGitTag failed: %v", err)
	}
	if tag != "v2.1.0" {
		t.Errorf("latestGitTag(2) = %q, want v2.1.0", tag)
	}
}

func TestLatestGitTagNoMatchingMajor(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v1.0.0")

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	_, err := latestGitTag(2)
	if err == nil {
		t.Fatal("expected error when no tags match target major")
	}
}

func TestLatestGitTagMixedPrefixes(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	runGit(t, dir, "tag", "v1.0.0")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "1.1.0")

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	tag, err := latestGitTag(1)
	if err != nil {
		t.Fatalf("latestGitTag failed: %v", err)
	}
	if tag != "1.1.0" {
		t.Errorf("latestGitTag(1) = %q, want 1.1.0 (1.1.0 is the later version despite no 'v' prefix)", tag)
	}
}

// TestLatestGitTagNumericMinorOrdering guards against lexicographic
// comparison: "v2.10.0" must outrank "v2.9.0" numerically, not sort before
// it because '1' < '9' as characters.
func TestLatestGitTagNumericMinorOrdering(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	runGit(t, dir, "tag", "v2.9.0")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "v2.10.0")

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	tag, err := latestGitTag(2)
	if err != nil {
		t.Fatalf("latestGitTag failed: %v", err)
	}
	if tag != "v2.10.0" {
		t.Errorf("latestGitTag(2) = %q, want v2.10.0 (numeric compare, not lexicographic)", tag)
	}
}

// TestLatestGitTagIgnoresReachability documents the intentional drop of
// branch-ancestry filtering: a tag reachable only from a side branch that
// was never merged into the production branch is still eligible.
func TestLatestGitTagIgnoresReachability(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	runGit(t, dir, "checkout", "-b", "other")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "never merged into main")
	runGit(t, dir, "tag", "v1.5.0")
	runGit(t, dir, "checkout", "main")

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	tag, err := latestGitTag(1)
	if err != nil {
		t.Fatalf("latestGitTag failed: %v", err)
	}
	if tag != "v1.5.0" {
		t.Errorf("latestGitTag(1) = %q, want v1.5.0 (tag selection is not branch-scoped)", tag)
	}
}

func TestLatestGitTagNoFilterReturnsAbsoluteLatest(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	runGit(t, dir, "tag", "v1.0.0")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "v3.0.0")

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	tag, err := latestGitTag(-1)
	if err != nil {
		t.Fatalf("latestGitTag failed: %v", err)
	}
	if tag != "v3.0.0" {
		t.Errorf("latestGitTag(-1) = %q, want v3.0.0 (no major filter, highest overall)", tag)
	}
}

func TestResolveDeployTagExplicitFlag(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v2.0.0")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		stackDir = origStackDir
	}()

	deployTag = "v2.0.0"
	deployMajor = false
	offline = false
	stackDir = dir

	tag, err := resolveDeployTag()
	if err != nil {
		t.Fatalf("resolveDeployTag failed: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %s", tag)
	}
}

func TestResolveDeployTagPinnedConfig(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v3.0.0")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = ""
	deployMajor = false
	offline = false
	cfg = &config.Config{
		Environment: config.EnvironmentConfig{
			Production: config.ProdConfig{
				Tag:    "v3.0.0",
				Branch: "main",
			},
		},
	}
	stackDir = dir

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
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = "v1.0.0"
	deployMajor = true
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

	// Dirty the tree: should NOT error when offline
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = "v1.0.0"
	deployMajor = true
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
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = "v1.0.0"
	deployMajor = false
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

func TestCurrentTagAtTaggedCommit(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v1.0.0")

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	tag := currentTag()
	if tag != "v1.0.0" {
		t.Errorf("currentTag = %q, want %q", tag, "v1.0.0")
	}
}

func TestCurrentTagOnBranch(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v1.0.0")

	// Make another commit so HEAD is past the tag
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	tag := currentTag()
	if tag != "" {
		t.Errorf("currentTag = %q, want empty (HEAD not at a tag)", tag)
	}
}

func TestCurrentTagDetachedAtTag(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v1.0.0")

	// Detach at tag
	runGit(t, dir, "checkout", "--detach", "v1.0.0")

	origStackDir := stackDir
	stackDir = dir
	defer func() { stackDir = origStackDir }()

	tag := currentTag()
	if tag != "v1.0.0" {
		t.Errorf("currentTag = %q, want %q", tag, "v1.0.0")
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

func TestResolveDeployTagAutoResolveStaysOnCurrentMajor(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	runGit(t, dir, "tag", "v2.0.0")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "v2.1.0")

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "third")
	runGit(t, dir, "tag", "v3.0.0")

	// Checkout v2.1.0 (simulating current deployed version)
	runGit(t, dir, "checkout", "--detach", "v2.1.0")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = ""
	deployMajor = false
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

	// Auto-resolve without --major should stay on current major (v2), not jump to v3.
	tag, err := resolveDeployTag()
	if err != nil {
		t.Fatalf("resolveDeployTag failed: %v", err)
	}
	if tag != "v2.1.0" {
		t.Errorf("expected v2.1.0 (latest v2, not v3), got %s", tag)
	}
}

func TestResolveDeployTagAllowsMajorWithFlag(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	runGit(t, dir, "tag", "v2.0.0")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "v2.1.0")

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "third")
	runGit(t, dir, "tag", "v3.0.0")

	runGit(t, dir, "checkout", "--detach", "v2.1.0")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = ""
	deployMajor = true
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
	if tag != "v3.0.0" {
		t.Errorf("expected v3.0.0, got %s", tag)
	}
}

func TestResolveDeployTagAllowsSameMajor(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	runGit(t, dir, "tag", "v2.0.0")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "v2.1.0")

	runGit(t, dir, "checkout", "--detach", "v2.0.0")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = ""
	deployMajor = false
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
	if tag != "v2.1.0" {
		t.Errorf("expected v2.1.0, got %s", tag)
	}
}

func TestResolveDeployTagExplicitTagBlocksMajorChange(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v2.0.0")

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "v3.0.0")

	// Detach at v2.0.0 so HEAD is at the v2 tag
	runGit(t, dir, "checkout", "--detach", "v2.0.0")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = "v3.0.0"
	deployMajor = false
	offline = true
	cfg = &config.Config{}
	stackDir = dir

	_, err := resolveDeployTag()
	if err == nil {
		t.Fatal("expected error: explicit --tag with major change without --major")
	}
}

func TestResolveDeployTagExplicitTagAllowsMajorWithFlag(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v2.0.0")

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "v3.0.0")

	// Detach at v2.0.0 so HEAD is at the v2 tag
	runGit(t, dir, "checkout", "--detach", "v2.0.0")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = "v3.0.0"
	deployMajor = true
	offline = true
	cfg = &config.Config{}
	stackDir = dir

	tag, err := resolveDeployTag()
	if err != nil {
		t.Fatalf("resolveDeployTag failed: %v", err)
	}
	if tag != "v3.0.0" {
		t.Errorf("expected v3.0.0, got %s", tag)
	}
}

func TestResolveDeployTagBlocksMultiStepMajor(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	runGit(t, dir, "tag", "v2.0.0")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "v4.0.0")

	// Detach at v2.0.0 so HEAD is at the v2 tag
	runGit(t, dir, "checkout", "--detach", "v2.0.0")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = "v4.0.0"
	deployMajor = true
	offline = true
	cfg = &config.Config{}
	stackDir = dir

	_, err := resolveDeployTag()
	if err == nil {
		t.Fatal("expected error: multi-step major jump even with --major")
	}
	if !strings.Contains(err.Error(), "skips") {
		t.Errorf("expected error mentioning 'skips', got: %v", err)
	}
}

func TestResolveDeployTagBlocksUndeterminedCurrentMajor(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v1.0.0")

	// Make another commit so HEAD is past the tag (undetermined current)
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = "v1.0.0"
	deployMajor = false
	offline = true
	cfg = &config.Config{}
	stackDir = dir

	_, err := resolveDeployTag()
	if err == nil {
		t.Fatal("expected error: undetermined current major without --major")
	}
}

func TestResolveDeployTagAllowsUndeterminedWithMajorFlag(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v1.0.0")

	// Make another commit so HEAD is past the tag (undetermined current)
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = "v1.0.0"
	deployMajor = true
	offline = true
	cfg = &config.Config{}
	stackDir = dir

	tag, err := resolveDeployTag()
	if err != nil {
		t.Fatalf("resolveDeployTag failed: %v", err)
	}
	if tag != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", tag)
	}
}

func TestResolveDeployTagAutoNoMajorTagExists(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v2.0.0")
	runGit(t, dir, "checkout", "--detach", "v2.0.0")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = ""
	deployMajor = true
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

	_, err := resolveDeployTag()
	if err == nil {
		t.Fatal("expected error: no v3 tag exists")
	}
}

func TestResolveDeployTagAllowsDowngradeWithMajor(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	runGit(t, dir, "tag", "v1.0.0")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "second")
	runGit(t, dir, "tag", "v2.0.0")

	runGit(t, dir, "checkout", "--detach", "v2.0.0")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	deployTag = "v1.0.0"
	deployMajor = true
	offline = true
	cfg = &config.Config{}
	stackDir = dir

	tag, err := resolveDeployTag()
	if err != nil {
		t.Fatalf("resolveDeployTag failed: %v", err)
	}
	if tag != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", tag)
	}
}

func TestResolveDeployTagNonSemverTagBypassesGuard(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)
	runGit(t, dir, "tag", "v2.0.0")
	runGit(t, dir, "checkout", "--detach", "v2.0.0")

	origDeployTag := deployTag
	origDeployMajor := deployMajor
	origOffline := offline
	origCfg := cfg
	origStackDir := stackDir
	defer func() {
		deployTag = origDeployTag
		deployMajor = origDeployMajor
		offline = origOffline
		cfg = origCfg
		stackDir = origStackDir
	}()

	// "latest" isn't semver: the guard can't compute a major for it, so it should deploy as given, with no --major required.
	deployTag = "latest"
	deployMajor = false
	offline = true
	cfg = &config.Config{}
	stackDir = dir

	tag, err := resolveDeployTag()
	if err != nil {
		t.Fatalf("resolveDeployTag failed: %v", err)
	}
	if tag != "latest" {
		t.Errorf("expected literal 'latest', got %s", tag)
	}
}

package giturl

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// setupWorktreeRepo creates a main repo with a "stack" subdirectory and a
// linked worktree nested under it (mirroring how dargstack worktrees are
// created under a project's .claude/worktrees/<branch> directory), and
// returns the main repo root and the worktree's "stack" subdirectory.
func setupWorktreeRepo(t *testing.T) (mainRepo, worktreeStackDir string) {
	t.Helper()

	parent := t.TempDir()
	mainRepo = filepath.Join(parent, "project")
	if err := os.MkdirAll(mainRepo, 0o755); err != nil {
		t.Fatal(err)
	}

	runGit(t, "", "init", mainRepo)
	runGit(t, mainRepo, "config", "user.email", "test@example.com")
	runGit(t, mainRepo, "config", "user.name", "Test")

	if err := os.MkdirAll(filepath.Join(mainRepo, "stack"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainRepo, "stack", "compose.yaml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, mainRepo, "add", "-A")
	runGit(t, mainRepo, "commit", "-m", "init")

	worktreeDir := filepath.Join(mainRepo, ".claude", "worktrees", "feat-test")
	if err := os.MkdirAll(filepath.Dir(worktreeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, mainRepo, "worktree", "add", "-b", "feat/test", worktreeDir)

	return mainRepo, filepath.Join(worktreeDir, "stack")
}

func TestSiblingParentDirResolvesWorktreeToMainCheckout(t *testing.T) {
	mainRepo, worktreeStackDir := setupWorktreeRepo(t)

	mainStackDir := filepath.Join(mainRepo, "stack")
	wantParent, err := filepath.EvalSymlinks(SiblingParentDir(mainStackDir))
	if err != nil {
		t.Fatal(err)
	}

	got, err := filepath.EvalSymlinks(SiblingParentDir(worktreeStackDir))
	if err != nil {
		t.Fatal(err)
	}

	if got != wantParent {
		t.Errorf("SiblingParentDir(worktree stack) = %q, want %q (main checkout)", got, wantParent)
	}

	worktreeParent := filepath.Dir(worktreeStackDir)
	if got == worktreeParent {
		t.Errorf("SiblingParentDir(worktree stack) = %q, should not resolve inside the worktree", got)
	}
}

func TestSiblingParentDirNonGitFallsBackToFilepathDir(t *testing.T) {
	stackDir := filepath.Join(t.TempDir(), "stack")
	if err := os.MkdirAll(stackDir, 0o755); err != nil {
		t.Fatal(err)
	}

	want := filepath.Dir(stackDir)
	if got := SiblingParentDir(stackDir); got != want {
		t.Errorf("SiblingParentDir(%q) = %q, want %q", stackDir, got, want)
	}
}

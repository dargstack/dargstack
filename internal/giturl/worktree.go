package giturl

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// SiblingParentDir returns the directory that sibling service repositories should be cloned into or resolved relative to.
//
// It is normally filepath.Dir(stackDir).
// But if stackDir sits inside a linked git worktree (for example one checked out under a project's .claude/worktrees/<branch> directory), that naive computation would resolve siblings relative to the worktree instead of the original project checkout.
// This maps stackDir onto the equivalent path in the worktree's main checkout first, so siblings always resolve the same way regardless of which worktree dargstack is run from.
func SiblingParentDir(stackDir string) string {
	return filepath.Dir(canonicalStackDir(stackDir))
}

// canonicalStackDir returns stackDir as it would appear under the main checkout of its git repository.
// If stackDir is not inside a linked worktree (or git metadata can't be read, e.g. in tests using a plain temp dir), it is returned unchanged.
func canonicalStackDir(stackDir string) string {
	// git resolves symlinks in the paths it reports (e.g. macOS's
	// /tmp -> /private/tmp), so resolve stackDir the same way before
	// comparing it against git's output below.
	resolvedStackDir := stackDir
	if resolved, err := filepath.EvalSymlinks(stackDir); err == nil {
		resolvedStackDir = resolved
	}

	worktreeRoot, err := gitOutput(resolvedStackDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return stackDir
	}

	commonDir, err := gitOutput(resolvedStackDir, "rev-parse", "--git-common-dir")
	if err != nil {
		return stackDir
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(resolvedStackDir, commonDir)
	}
	mainRoot := filepath.Dir(commonDir)

	if mainRoot == worktreeRoot {
		return stackDir
	}

	relPath, err := filepath.Rel(worktreeRoot, resolvedStackDir)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return stackDir
	}

	return filepath.Join(mainRoot, relPath)
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

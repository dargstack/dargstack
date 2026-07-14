package giturl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloneWithFallback_NoURL(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "repo")

	err := CloneWithFallback(GitURL{}, target)
	if err == nil {
		t.Error("CloneWithFallback with empty GitURL should return an error")
	}
}

func TestCloneWithFallback_HttpsOnly_NoAgent(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "repo")

	oldSock := os.Getenv("SSH_AUTH_SOCK")
	_ = os.Unsetenv("SSH_AUTH_SOCK")
	t.Cleanup(func() { _ = os.Setenv("SSH_AUTH_SOCK", oldSock) })

	httpsURL := "file://" + filepath.ToSlash(filepath.Join(tmpDir, "does-not-exist"))
	err := CloneWithFallback(GitURL{HTTPS: httpsURL}, target)
	if err == nil {
		t.Fatal("CloneWithFallback should fail for nonexistent repo")
	}
	if got := err.Error(); len(got) < len("clone ")+len(httpsURL) || got[:len("clone ")+len(httpsURL)] != "clone "+httpsURL {
		t.Fatalf("expected error to reference attempted URL %q, got %q", httpsURL, got)
	}
}

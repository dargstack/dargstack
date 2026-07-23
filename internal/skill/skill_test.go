package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundledContent(t *testing.T) {
	content := BundledContent()
	if content == "" {
		t.Fatal("bundled content is empty")
	}
	if content[0] != '-' {
		t.Fatal("content does not start with frontmatter delimiter")
	}
}

func TestBundledHash(t *testing.T) {
	hash := BundledHash()
	if hash == "" {
		t.Fatal("bundled hash is empty")
	}
	// sha256 hex is 64 characters
	if len(hash) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash))
	}
}

func TestSkillPathGlobal(t *testing.T) {
	path, err := SkillPath(false)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %s", path)
	}
	expected := filepath.Join(os.Getenv("HOME"), ".agents", "skills", "dargstack")
	resolved, _ := filepath.EvalSymlinks(path)
	resolvedExpected, _ := filepath.EvalSymlinks(expected)
	if resolved != resolvedExpected {
		t.Errorf("expected %s, got %s", resolvedExpected, resolved)
	}
}

func TestSkillPathProject(t *testing.T) {
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	path, err := SkillPath(true)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(tmp, ".agents", "skills", "dargstack")
	resolvedPath, _ := filepath.EvalSymlinks(path)
	resolvedExpected, _ := filepath.EvalSymlinks(expected)
	if resolvedPath != resolvedExpected {
		t.Errorf("expected %s, got %s", resolvedExpected, resolvedPath)
	}
}

func TestInstallAndIsInstalled(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "dargstack")

	if IsInstalled(skillDir) {
		t.Fatal("expected not installed before install")
	}

	updated, modified, err := Install(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected updated=true on first install")
	}
	if modified {
		t.Fatal("expected modified=false on first install")
	}
	if !IsInstalled(skillDir) {
		t.Fatal("expected installed after install")
	}

	// Second install should be no-op.
	updated, modified, err = Install(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("expected updated=false when already current")
	}
	if modified {
		t.Fatal("expected modified=false when already current")
	}
}

func TestInstallDetectsUserModification(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "dargstack")

	if _, _, err := Install(skillDir); err != nil {
		t.Fatal(err)
	}

	// Modify the skill file.
	skillPath := SkillFilePath(skillDir)
	if err := os.WriteFile(skillPath, []byte("# user edited"), 0o644); err != nil {
		t.Fatal(err)
	}

	updated, modified, err := Install(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("expected updated=false when user modified")
	}
	if !modified {
		t.Fatal("expected modified=true when user modified")
	}
}

func TestUninstall(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "dargstack")

	if _, _, err := Install(skillDir); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall(skillDir); err != nil {
		t.Fatal(err)
	}
	if IsInstalled(skillDir) {
		t.Fatal("expected not installed after uninstall")
	}
}

func TestUninstallNotInstalled(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "dargstack")

	err := Uninstall(skillDir)
	if err == nil {
		t.Fatal("expected error when uninstalling non-installed skill")
	}
}

func TestUpdateNotInstalled(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "dargstack")

	updated, err := Update(skillDir)
	if err == nil {
		t.Fatal("expected error when updating non-installed skill")
	}
	if updated {
		t.Fatal("expected updated=false on error")
	}
}

func TestUpdateInstalled(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "dargstack")

	if _, _, err := Install(skillDir); err != nil {
		t.Fatal(err)
	}

	updated, err := Update(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("expected updated=false when already current")
	}
}

func TestStatusNotInstalled(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "dargstack")

	info, err := Status(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Installed {
		t.Fatal("expected not installed")
	}
	if info.UpToDate {
		t.Fatal("expected not up to date")
	}
}

func TestStatusInstalled(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "dargstack")

	if _, _, err := Install(skillDir); err != nil {
		t.Fatal(err)
	}

	info, err := Status(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Installed {
		t.Fatal("expected installed")
	}
	if !info.UpToDate {
		t.Fatal("expected up to date")
	}
	if info.UserModified {
		t.Fatal("expected not user modified")
	}
}

func TestStatusUserModified(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "dargstack")

	if _, _, err := Install(skillDir); err != nil {
		t.Fatal(err)
	}

	// Modify the skill file.
	if err := os.WriteFile(SkillFilePath(skillDir), []byte("# edited"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := Status(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.UserModified {
		t.Fatal("expected user modified")
	}
}

func TestReadMeta(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "dargstack")

	// Nonexistent.
	meta, err := ReadMeta(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if meta != nil {
		t.Fatal("expected nil meta for nonexistent file")
	}

	// After install.
	if _, _, err := Install(skillDir); err != nil {
		t.Fatal(err)
	}
	meta, err = ReadMeta(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if meta == nil {
		t.Fatal("expected non-nil meta after install")
	}
	if meta.SHA256 != BundledHash() {
		t.Errorf("meta hash mismatch: %s != %s", meta.SHA256, BundledHash())
	}
}

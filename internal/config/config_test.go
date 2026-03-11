package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	content := `compatibility: ">=1.0.0 <2.0.0"
name: "teststack"
sudo: "never"
production:
  branch: "main"
  tag: "latest"
`
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Name != "teststack" {
		t.Errorf("expected name=teststack, got %s", cfg.Name)
	}
	if cfg.Sudo != "never" {
		t.Errorf("expected sudo=never, got %s", cfg.Sudo)
	}
	if cfg.Production.Branch != "main" {
		t.Errorf("expected branch=main, got %s", cfg.Production.Branch)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	content := ""
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Sudo != "auto" {
		t.Errorf("expected default sudo=auto, got %s", cfg.Sudo)
	}
	if cfg.Production.Branch != "main" {
		t.Errorf("expected default branch=main, got %s", cfg.Production.Branch)
	}
	if cfg.Production.Tag != "latest" {
		t.Errorf("expected default tag=latest, got %s", cfg.Production.Tag)
	}
}

func TestDetectStackDir(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}

	found, err := DetectStackDir()
	if err != nil {
		t.Fatal(err)
	}
	if found != dir {
		t.Errorf("expected %s, got %s", dir, found)
	}
}

func TestDomainDefault(t *testing.T) {
	dir := t.TempDir()
	content := ""
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Production.Domain != "app.localhost" {
		t.Errorf("expected default production.domain=app.localhost, got %s", cfg.Production.Domain)
	}
}

func TestDomainCustom(t *testing.T) {
	dir := t.TempDir()
	content := "production:\n  domain: myapp.example.com\n"
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Production.Domain != "myapp.example.com" {
		t.Errorf("expected production.domain=myapp.example.com, got %s", cfg.Production.Domain)
	}
}

func TestCollectServiceFilesDir(t *testing.T) {
	dir := t.TempDir()

	// Create service directories with compose.yaml
	for _, name := range []string{"api", "postgres", "web"} {
		svcDir := filepath.Join(dir, name)
		if err := os.MkdirAll(svcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(svcDir, "compose.yaml"), []byte("services:\n  "+name+":\n    image: test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A file at the top level should be ignored (not a service directory)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("KEY=VALUE"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A directory without compose.yaml should be ignored
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	files, err := CollectServiceFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 compose files, got %d: %v", len(files), files)
	}
}

func TestCollectServiceFilesNonexistent(t *testing.T) {
	files, err := CollectServiceFiles("/nonexistent/services")
	if err != nil {
		t.Fatal("expected no error for nonexistent directory")
	}
	if files != nil {
		t.Errorf("expected nil for nonexistent directory, got %v", files)
	}
}

func TestPathHelpers(t *testing.T) {
	stackDir := "/project/stack"

	tests := []struct {
		name string
		fn   func(string) string
		want string
	}{
		{"DevDir", DevDir, "/project/stack/src/development"},
		{"ProdDir", ProdDir, "/project/stack/src/production"},
		{"ArtifactsDir", ArtifactsDir, "/project/stack/artifacts"},
		{"CertificatesDir", CertificatesDir, "/project/stack/artifacts/certificates"},
		{"SecretsDir", SecretsDir, "/project/stack/artifacts/secrets"},
		{"DevEnvFile", DevEnvFile, "/project/stack/src/development/.env"},
		{"ProdEnvFile", ProdEnvFile, "/project/stack/src/production/.env"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn(stackDir)
			if got != tt.want {
				t.Errorf("%s(%s) = %s, want %s", tt.name, stackDir, got, tt.want)
			}
		})
	}
}

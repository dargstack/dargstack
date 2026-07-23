package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	content := `metadata:
  compatibility: ">=1.0.0 <2.0.0"
  name: teststack
runtime:
  sudo: never
environment:
  production:
    branch: main
    tag: latest
`
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Metadata.Name != "teststack" {
		t.Errorf("expected name=teststack, got %s", cfg.Metadata.Name)
	}
	if cfg.Runtime.Sudo != SudoNever {
		t.Errorf("expected sudo=never, got %s", cfg.Runtime.Sudo)
	}
	if cfg.Environment.Production.Branch != "main" {
		t.Errorf("expected branch=main, got %s", cfg.Environment.Production.Branch)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Runtime.Sudo != SudoAuto {
		t.Errorf("expected default sudo=auto, got %s", cfg.Runtime.Sudo)
	}
	if cfg.Runtime.Build.Mode != BuildAlways {
		t.Errorf("expected default build mode=always, got %s", cfg.Runtime.Build.Mode)
	}
	if cfg.Environment.Production.Branch != "main" {
		t.Errorf("expected default branch=main, got %s", cfg.Environment.Production.Branch)
	}
	if cfg.Environment.Production.Tag != "" {
		t.Errorf("expected default tag to be empty, got %s", cfg.Environment.Production.Tag)
	}
}

func TestDomainDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Environment.Production.Domain != "app.localhost" {
		t.Errorf("expected default production.domain=app.localhost, got %s", cfg.Environment.Production.Domain)
	}
	if cfg.Environment.Development.Domain != "app.localhost" {
		t.Errorf("expected default development.domain=app.localhost, got %s", cfg.Environment.Development.Domain)
	}
}

func TestBuildMode(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantMode BuildMode
	}{
		{"mode always", "runtime:\n  build:\n    mode: always\n", BuildAlways},
		{"mode missing", "runtime:\n  build:\n    mode: missing\n", BuildMissing},
		{"no build config", "", BuildAlways},
		{"empty build config", "runtime:\n  build:\n", BuildAlways},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Runtime.Build.Mode != tt.wantMode {
				t.Errorf("expected mode=%q, got %q", tt.wantMode, cfg.Runtime.Build.Mode)
			}
		})
	}
}

func TestVolumePrompt(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantPrompt bool
	}{
		{"prompt true", "runtime:\n  deploy:\n    volumes:\n      prompt: true\n", true},
		{"prompt false", "runtime:\n  deploy:\n    volumes:\n      prompt: false\n", false},
		{"no deploy config", "", true},
		{"empty deploy config", "runtime:\n  deploy:\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			got := *cfg.Runtime.Deploy.Volumes.Prompt
			if got != tt.wantPrompt {
				t.Errorf("expected prompt=%v, got %v", tt.wantPrompt, got)
			}
		})
	}
}

func TestSudoModeInvalid(t *testing.T) {
	dir := t.TempDir()
	content := "runtime:\n  sudo: invalid\n"
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid sudo mode")
	}
}

func TestBuildModeInvalid(t *testing.T) {
	dir := t.TempDir()
	content := "runtime:\n  build:\n    mode: invalid\n"
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid build mode")
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
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	resolvedFound, err := filepath.EvalSymlinks(found)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedFound != resolvedDir {
		t.Errorf("expected %s, got %s", resolvedDir, resolvedFound)
	}
}

func TestPathHelpers(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"StackDir", cfg.StackDir(), resolved},
		{"DevDir", cfg.DevDir(), filepath.Join(resolved, "src", "development")},
		{"ProdDir", cfg.ProdDir(), filepath.Join(resolved, "src", "production")},
		{"ArtifactsDir", cfg.ArtifactsDir(), filepath.Join(resolved, "artifacts")},
		{"CertificatesDir", cfg.CertificatesDir(), filepath.Join(resolved, "artifacts", "certificates")},
		{"SecretsDir", cfg.SecretsDir(), filepath.Join(resolved, "artifacts", "secrets")},
		{"DevEnvFile", cfg.DevEnvFile(), filepath.Join(resolved, "src", "development", ".env")},
		{"ProdEnvFile", cfg.ProdEnvFile(), filepath.Join(resolved, "src", "production", ".env")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %s, want %s", tt.got, tt.want)
			}
		})
	}
}

func TestStackDirResolvesRelativePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StackDir() != resolvedDir {
		t.Errorf("expected %s, got %s", resolvedDir, cfg.StackDir())
	}
	if !filepath.IsAbs(cfg.StackDir()) {
		t.Errorf("StackDir should be absolute, got %s", cfg.StackDir())
	}
}

func TestCollectServiceFilesDir(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"api", "postgres", "web"} {
		svcDir := filepath.Join(dir, name)
		if err := os.MkdirAll(svcDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(svcDir, "compose.yaml"), []byte("services:\n  "+name+":\n    image: test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("KEY=VALUE"), 0o644); err != nil {
		t.Fatal(err)
	}
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

func TestCustomDomainOverride(t *testing.T) {
	dir := t.TempDir()
	content := `environment:
  development:
    domain: dev.example.com
  production:
    domain: prod.example.com
`
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Environment.Development.Domain != "dev.example.com" {
		t.Errorf("expected dev domain=dev.example.com, got %s", cfg.Environment.Development.Domain)
	}
	if cfg.Environment.Production.Domain != "prod.example.com" {
		t.Errorf("expected prod domain=prod.example.com, got %s", cfg.Environment.Production.Domain)
	}
}

func TestCertificateIncludeExclude(t *testing.T) {
	dir := t.TempDir()
	content := `environment:
   development:
     certificate:
       exclude:
         - admin.app.localhost
       include:
         - foo.example.com
         - bar.local
`
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Environment.Development.Certificate.Exclude) != 1 {
		t.Fatalf("expected 1 exclude, got %d", len(cfg.Environment.Development.Certificate.Exclude))
	}
	if cfg.Environment.Development.Certificate.Exclude[0] != "admin.app.localhost" {
		t.Errorf("expected exclude=admin.app.localhost, got %s", cfg.Environment.Development.Certificate.Exclude[0])
	}
	if len(cfg.Environment.Development.Certificate.Include) != 2 {
		t.Fatalf("expected 2 include, got %d", len(cfg.Environment.Development.Certificate.Include))
	}
}

func TestDetectStackDirIn(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "stack", "src", "development")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stack", ConfigFileName), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := DetectStackDirIn(subDir)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(dir, "stack")
	resolvedFound, _ := filepath.EvalSymlinks(found)
	resolvedExpected, _ := filepath.EvalSymlinks(expected)
	if resolvedFound != resolvedExpected {
		t.Errorf("expected %s, got %s", resolvedExpected, resolvedFound)
	}
}

func TestDetectStackDirInNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := DetectStackDirIn(dir)
	if err == nil {
		t.Fatal("expected error when config not found")
	}
}

func TestSkillInstallModeUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    SkillInstallMode
		wantErr bool
	}{
		{"bool true", "runtime:\n  skill:\n    install: true\n", SkillInstallAuto, false},
		{"bool false", "runtime:\n  skill:\n    install: false\n", SkillInstallOff, false},
		{"string auto", "runtime:\n  skill:\n    install: auto\n", SkillInstallAuto, false},
		{"string once", "runtime:\n  skill:\n    install: once\n", SkillInstallOnce, false},
		{"string off", "runtime:\n  skill:\n    install: off\n", SkillInstallOff, false},
		{"invalid string", "runtime:\n  skill:\n    install: invalid\n", "", true},
		{"default", "", SkillInstallAuto, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(dir)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Runtime.Skill.Install != tt.want {
				t.Errorf("expected %q, got %q", tt.want, cfg.Runtime.Skill.Install)
			}
		})
	}
}

func TestGlobalConfigLoad(t *testing.T) {
	// Test with nonexistent file — should return defaults.
	origHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", origHome) }()

	tmp := t.TempDir()
	_ = os.Setenv("HOME", tmp)

	cfg, err := LoadGlobalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runtime.Skill.Install != SkillInstallAuto {
		t.Errorf("expected default auto, got %q", cfg.Runtime.Skill.Install)
	}
}

func TestGlobalConfigLoadWithFile(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", origHome) }()

	tmp := t.TempDir()
	_ = os.Setenv("HOME", tmp)

	configDir := filepath.Join(tmp, ".config", "dargstack")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := "runtime:\n  skill:\n    install: once\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadGlobalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runtime.Skill.Install != SkillInstallOnce {
		t.Errorf("expected once, got %q", cfg.Runtime.Skill.Install)
	}
}

func TestEffectiveSkillInstall(t *testing.T) {
	global := &GlobalConfig{}
	global.applyDefaults()
	project := &Config{}
	project.applyDefaults()

	// Both defaults — should return auto.
	if EffectiveSkillInstall(global, project) != SkillInstallAuto {
		t.Error("expected auto when both are defaults")
	}

	// Global is once, project is default — should return once.
	global.Runtime.Skill.Install = SkillInstallOnce
	project.Runtime.Skill.Install = SkillInstallAuto
	if EffectiveSkillInstall(global, project) != SkillInstallOnce {
		t.Error("expected once when global is once and project is default")
	}

	// Global is auto, project is off — project wins.
	global.Runtime.Skill.Install = SkillInstallAuto
	project.Runtime.Skill.Install = SkillInstallOff
	if EffectiveSkillInstall(global, project) != SkillInstallOff {
		t.Error("expected off when project overrides global")
	}
}

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/secret"
)

func TestSelectConfigOrphans(t *testing.T) {
	declared := map[string]bool{"prometheus-yaml-a1b2c3d4": true, "plain-config": true}
	listed := []string{
		"prometheus-yaml-a1b2c3d4", // declared: keep
		"prometheus-yaml-11112222", // suffixed, undeclared: orphan
		"plain-config",             // undeclared but no suffix: keep
		"other-33334444",           // suffixed, undeclared: orphan
	}
	got := selectConfigOrphans(listed, declared)
	if len(got) != 2 || got[0] != "other-33334444" || got[1] != "prometheus-yaml-11112222" {
		t.Fatalf("unexpected orphans: %v", got)
	}

	if got := selectConfigOrphans(nil, declared); len(got) != 0 {
		t.Fatalf("expected no orphans, got %v", got)
	}
}

func TestApplyDerivedConfigHashSuffixes(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "signing-key-pub")
	content := []byte("-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----\n")
	if err := os.WriteFile(file, content, 0o644); err != nil {
		t.Fatal(err)
	}

	composeYAML := `configs:
  signing-key-pub:
    file: ` + file + `
x-dargstack:
  configs:
    signing-key-pub:
      type: public_key
      source: signing-key
services:
  api:
    image: api:latest
    configs:
      - signing-key-pub
`
	fullYAML := composeYAML + `
  other:
    image: other:latest
    configs:
      - source: signing-key-pub
`

	hash := secret.ConfigContentHash(content)
	newName := "signing-key-pub-" + hash

	deployed, full, err := applyDerivedConfigHashSuffixes([]byte(composeYAML), []byte(fullYAML))
	if err != nil {
		t.Fatal(err)
	}

	for label, data := range map[string][]byte{"deployed": deployed, "full": full} {
		var doc map[string]interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			t.Fatalf("%s: invalid YAML: %v", label, err)
		}
		configs := doc["configs"].(map[string]interface{})
		if _, ok := configs["signing-key-pub"]; ok {
			t.Errorf("%s: old top-level key must be gone", label)
		}
		def, ok := configs[newName].(map[string]interface{})
		if !ok {
			t.Errorf("%s: renamed top-level key missing: %s", label, data)
			continue
		}
		if def["file"] != filepath.Join(dir, newName) {
			t.Errorf("%s: expected rewritten file path, got %v", label, def["file"])
		}
	}

	// Short and long form references must be renamed.
	var doc map[string]interface{}
	if err := yaml.Unmarshal(deployed, &doc); err != nil {
		t.Fatal(err)
	}
	api := doc["services"].(map[string]interface{})["api"].(map[string]interface{})
	if refs := api["configs"].([]interface{}); refs[0] != newName {
		t.Errorf("unexpected short-form reference: %v", refs)
	}
	otherDoc := map[string]interface{}{}
	if err := yaml.Unmarshal(full, &otherDoc); err != nil {
		t.Fatal(err)
	}
	other := otherDoc["services"].(map[string]interface{})["other"].(map[string]interface{})
	if longForm := other["configs"].([]interface{})[0].(map[string]interface{}); longForm["source"] != newName {
		t.Errorf("unexpected long-form reference: %v", longForm)
	}

	// The artifact file must be renamed to match the deployed name.
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("base artifact should be gone, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, newName)); err != nil {
		t.Errorf("renamed artifact should exist: %v", err)
	}
}

func TestApplyDerivedConfigHashSuffixesNoTemplates(t *testing.T) {
	in := []byte("configs:\n  a:\n    file: /x/a\n")
	deployed, full, err := applyDerivedConfigHashSuffixes(in, in)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(deployed, in) || !bytes.Equal(full, in) {
		t.Error("compose without x-dargstack.configs must be untouched")
	}
}

// withTestStack sets the package-level stack globals to a temporary dargstack project and
// restores them on cleanup.
func withTestStack(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dargstack.yaml"), []byte("metadata:\n  name: teststack\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load test config: %v", err)
	}

	origCfg, origStackDir, origEnv, origDryRun := cfg, stackDir, env, dryRun
	cfg = loaded
	stackDir = dir
	env = ""
	dryRun = false
	t.Cleanup(func() { cfg, stackDir, env, dryRun = origCfg, origStackDir, origEnv, origDryRun })
}

func writeRotateFixture(t *testing.T) (composePath, cfgPath string) {
	t.Helper()
	devSvcDir := filepath.Join(stackDir, "src", "development", "prometheus")
	if err := os.MkdirAll(devSvcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	compose := `services:
  prometheus:
    image: prom/prometheus:latest
    configs:
      - prometheus-yaml-aaaaaaaa
configs:
  prometheus-yaml-aaaaaaaa:
    file: ./prometheus.yaml
`
	if err := os.WriteFile(filepath.Join(devSvcDir, "compose.yaml"), []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath = filepath.Join(devSvcDir, "prometheus.yaml")
	if err := os.WriteFile(cfgPath, []byte("global:\n  scrape_interval: 30s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(devSvcDir, "compose.yaml"), cfgPath
}

func TestConfigRotate(t *testing.T) {
	withTestStack(t)
	_, cfgPath := writeRotateFixture(t)
	hash := secret.ConfigContentHash([]byte("global:\n  scrape_interval: 30s\n"))
	newName := "prometheus-yaml-" + hash

	if err := runConfigRotate(nil, []string{"prometheus-yaml-aaaaaaaa"}); err != nil {
		t.Fatalf("runConfigRotate: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(stackDir, "src", "development", "prometheus", "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, newName+":") {
		t.Errorf("expected renamed top-level key %s in:\n%s", newName, got)
	}
	if !strings.Contains(got, "- "+newName) {
		t.Errorf("expected renamed service reference in:\n%s", got)
	}
	if strings.Contains(got, "prometheus-yaml-aaaaaaaa") {
		t.Errorf("old name must be gone:\n%s", got)
	}
	// The committed content file is not renamed: it is the source of truth.
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("content file must survive rotation: %v", err)
	}
}

func TestConfigRotateBaseNameArgument(t *testing.T) {
	withTestStack(t)
	writeRotateFixture(t)
	hash := secret.ConfigContentHash([]byte("global:\n  scrape_interval: 30s\n"))
	newName := "prometheus-yaml-" + hash

	if err := runConfigRotate(nil, []string{"prometheus-yaml"}); err != nil {
		t.Fatalf("runConfigRotate: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(stackDir, "src", "development", "prometheus", "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), newName+":") {
		t.Errorf("expected renamed top-level key %s in:\n%s", newName, data)
	}
}

func TestConfigRotateAlreadyUpToDate(t *testing.T) {
	withTestStack(t)
	writeRotateFixture(t)
	hash := secret.ConfigContentHash([]byte("global:\n  scrape_interval: 30s\n"))
	newName := "prometheus-yaml-" + hash

	if err := runConfigRotate(nil, []string{"prometheus-yaml-aaaaaaaa"}); err != nil {
		t.Fatalf("first rotate: %v", err)
	}
	composePath := filepath.Join(stackDir, "src", "development", "prometheus", "compose.yaml")
	if err := runConfigRotate(nil, []string{newName}); err != nil {
		t.Fatalf("second rotate should be a no-op, got: %v", err)
	}
	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), newName+":") != 1 {
		t.Errorf("expected exactly one top-level key %s:\n%s", newName, data)
	}
}

func TestConfigRotateNotFound(t *testing.T) {
	withTestStack(t)
	writeRotateFixture(t)
	err := runConfigRotate(nil, []string{"does-not-exist"})
	if err == nil {
		t.Fatal("expected error for unknown config")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error should name the missing config: %v", err)
	}
}

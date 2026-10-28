package secret

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestConfigContentHash(t *testing.T) {
	// sha256("hello\n") = 5891b5b528d5c11d36ca3a9f6cdb444c...
	if got := ConfigContentHash([]byte("hello\n")); got != "5891b5b5" {
		t.Errorf("expected 5891b5b5, got %s", got)
	}
	if len(ConfigContentHash([]byte("anything"))) != ConfigHashLen {
		t.Errorf("expected hash of length %d", ConfigHashLen)
	}
}

func TestSplitConfigName(t *testing.T) {
	tests := []struct {
		name string
		base string
		hash string
		ok   bool
	}{
		{name: "prometheus-yaml-a1b2c3d4", base: "prometheus-yaml", hash: "a1b2c3d4", ok: true},
		{name: "foo-deadbeef", base: "foo", hash: "deadbeef", ok: true},
		{name: "a-b-c1d2e3f4", base: "a-b", hash: "c1d2e3f4", ok: true},
		{name: "foo", ok: false},
		{name: "foo-1234567g", ok: false},  // not hex
		{name: "foo-1234567", ok: false},   // 7 chars
		{name: "foo-123456789", ok: false}, // 9 chars
		{name: "-12345678", ok: false},     // empty base
		{name: "foo-ABCDEF12", ok: false},  // uppercase is not a suffix
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, hash, ok := SplitConfigName(tt.name)
			if ok != tt.ok {
				t.Fatalf("expected ok=%v, got %v (base=%q hash=%q)", tt.ok, ok, base, hash)
			}
			if tt.ok && (base != tt.base || hash != tt.hash) {
				t.Errorf("expected (%q, %q), got (%q, %q)", tt.base, tt.hash, base, hash)
			}
			if HasConfigHashSuffix(tt.name) != tt.ok {
				t.Errorf("HasConfigHashSuffix(%q) = %v, want %v", tt.name, !tt.ok, tt.ok)
			}
		})
	}
}

func TestWithConfigHashSuffix(t *testing.T) {
	if got := WithConfigHashSuffix("prometheus-yaml", "a1b2c3d4"); got != "prometheus-yaml-a1b2c3d4" {
		t.Errorf("unexpected result: %s", got)
	}
}

func TestDerivedConfigHashSuffixes(t *testing.T) {
	dir := t.TempDir()

	// Written content for the first config.
	content := "key: value\n"
	file := filepath.Join(dir, "derived-a")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Leftover artifact for the second config (no file at the base path).
	leftoverContent := "leftover bytes\n"
	leftover := filepath.Join(dir, "derived-b-aaaaaaaa")
	if err := os.WriteFile(leftover, []byte(leftoverContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Third config: no file and no leftover.

	composeYAML := `configs:
  derived-a:
    file: ` + file + `
  derived-b:
    file: ` + filepath.Join(dir, "derived-b") + `
  derived-c:
    file: ` + filepath.Join(dir, "derived-c") + `
  derived-d-11112222:
    file: ` + filepath.Join(dir, "derived-d-11112222") + `
x-dargstack:
  configs:
    derived-a:
      type: public_key
      source: some-key
    derived-b:
      type: public_key
      source: some-key
    derived-c:
      type: public_key
      source: some-key
    derived-d-11112222:
      type: public_key
      source: some-key
`

	suffixes, err := DerivedConfigHashSuffixes([]byte(composeYAML))
	if err != nil {
		t.Fatal(err)
	}

	if got := suffixes["derived-a"]; got != ConfigContentHash([]byte(content)) {
		t.Errorf("derived-a: expected hash of written content, got %s", got)
	}
	if got := suffixes["derived-b"]; got != ConfigContentHash([]byte(leftoverContent)) {
		t.Errorf("derived-b: expected hash of leftover content, got %s", got)
	}
	if _, ok := suffixes["derived-c"]; ok {
		t.Errorf("derived-c: expected no suffix when content is unavailable")
	}
	if _, ok := suffixes["derived-d-11112222"]; ok {
		t.Errorf("derived-d: expected no suffix when the name already carries one")
	}
	if len(suffixes) != 2 {
		t.Errorf("expected 2 suffixes, got %v", suffixes)
	}
}

func TestRenameConfigsWithSuffix(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "derived-a")

	composeYAML := `configs:
  derived-a:
    file: ` + file + `
  unrelated:
    file: ` + filepath.Join(dir, "unrelated") + `
services:
  api:
    image: api:latest
    configs:
      - derived-a
      - unrelated
  verifier:
    image: verifier:latest
    configs:
      - source: derived-a
        target: /etc/derived-a.pub
    environment:
      - DERIVED_A_REF=derived-a
`

	out, err := RenameConfigsWithSuffix([]byte(composeYAML), map[string]string{"derived-a": "b2c3d4e5"})
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("result is not valid YAML: %v\n%s", err, out)
	}

	configs := doc["configs"].(map[string]interface{})
	if _, ok := configs["derived-a"]; ok {
		t.Error("old top-level key must be gone")
	}
	newDef, ok := configs["derived-a-b2c3d4e5"].(map[string]interface{})
	if !ok {
		t.Fatalf("renamed top-level config key missing:\n%s", out)
	}
	if newDef["file"] != filepath.Join(dir, "derived-a-b2c3d4e5") {
		t.Errorf("expected rewritten file path, got %v", newDef["file"])
	}
	if _, ok := configs["unrelated"]; !ok {
		t.Error("unrelated config must keep its name")
	}

	services := doc["services"].(map[string]interface{})
	api := services["api"].(map[string]interface{})
	apiConfigs := api["configs"].([]interface{})
	if apiConfigs[0] != "derived-a-b2c3d4e5" || apiConfigs[1] != "unrelated" {
		t.Errorf("unexpected api configs: %v", apiConfigs)
	}
	verifier := services["verifier"].(map[string]interface{})
	vConfigs := verifier["configs"].([]interface{})
	longForm := vConfigs[0].(map[string]interface{})
	if longForm["source"] != "derived-a-b2c3d4e5" || longForm["target"] != "/etc/derived-a.pub" {
		t.Errorf("unexpected long-form reference: %v", longForm)
	}
	env := verifier["environment"].([]interface{})
	if env[0] != "DERIVED_A_REF=derived-a" {
		t.Errorf("environment values must not be renamed: %v", env)
	}
}

func TestRenameConfigsWithSuffixEmpty(t *testing.T) {
	in := []byte("configs:\n  a:\n    file: /x/a\n")
	out, err := RenameConfigsWithSuffix(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("expected unchanged bytes, got %s", out)
	}
}

func TestMoveConfigArtifacts(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "derived-a")
	if err := os.WriteFile(base, []byte("fresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "derived-a-aaaaaaaa")
	if err := os.WriteFile(stale, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	moved, err := MoveConfigArtifacts(map[string]string{"derived-a": base}, map[string]string{"derived-a": "b2c3d4e5"})
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 1 || moved[0] != filepath.Join(dir, "derived-a-b2c3d4e5") {
		t.Errorf("unexpected moved files: %v", moved)
	}
	if _, err := os.Stat(base); !os.IsNotExist(err) {
		t.Errorf("base file should be gone, stat err=%v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale file should be gone, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "derived-a-b2c3d4e5")); err != nil {
		t.Errorf("renamed file should exist: %v", err)
	}
}

func TestMoveConfigArtifactsLeftoverInPlace(t *testing.T) {
	dir := t.TempDir()
	// No file at the base path; the leftover already sits at the destination path.
	leftover := filepath.Join(dir, "derived-a-b2c3d4e5")
	if err := os.WriteFile(leftover, []byte("leftover\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	moved, err := MoveConfigArtifacts(map[string]string{"derived-a": filepath.Join(dir, "derived-a")}, map[string]string{"derived-a": "b2c3d4e5"})
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 0 {
		t.Errorf("expected no moves, got %v", moved)
	}
	if _, err := os.Stat(leftover); err != nil {
		t.Errorf("leftover must survive when it is the deploy target: %v", err)
	}
}

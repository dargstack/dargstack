package secret

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractConfigTemplates(t *testing.T) {
	composeYAML := `services:
  api:
    image: api:latest
x-dargstack:
  configs:
    signing-key-pub:
      type: public_key
      source: signing-key
`

	templates, err := ExtractConfigTemplates([]byte(composeYAML))
	if err != nil {
		t.Fatal(err)
	}

	if len(templates) != 1 {
		t.Fatalf("expected 1 config template, got %d", len(templates))
	}

	tmpl := templates["signing-key-pub"]
	if tmpl.Type != TypePublicKey {
		t.Errorf("expected type=%s, got %s", TypePublicKey, tmpl.Type)
	}
	if tmpl.Source != "signing-key" {
		t.Errorf("expected source=signing-key, got %s", tmpl.Source)
	}
}

func TestExtractConfigTemplatesInferredType(t *testing.T) {
	composeYAML := `x-dargstack:
  configs:
    signing-key-pub:
      source: signing-key
`
	templates, err := ExtractConfigTemplates([]byte(composeYAML))
	if err != nil {
		t.Fatal(err)
	}
	if templates["signing-key-pub"].Type != TypePublicKey {
		t.Errorf("expected inferred type=%s, got %s", TypePublicKey, templates["signing-key-pub"].Type)
	}
}

func TestExtractConfigTemplatesNoExtension(t *testing.T) {
	composeYAML := `services:
  api:
    image: api
`
	templates, err := ExtractConfigTemplates([]byte(composeYAML))
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 0 {
		t.Errorf("expected 0 templates, got %d", len(templates))
	}
}

func TestExtractConfigPaths(t *testing.T) {
	composeYAML := `configs:
  signing-key-pub:
    file: ./secrets/signing-key-pub.pub
`
	paths := ExtractConfigPaths([]byte(composeYAML))
	if paths["signing-key-pub"] != "./secrets/signing-key-pub.pub" {
		t.Errorf("unexpected path: %v", paths)
	}
}

func TestRewriteConfigFilePaths(t *testing.T) {
	composeYAML := `configs:
  signing-key-pub:
    file: /abs/path/signing-key-pub.pub
`
	configsDir := filepath.Join(string(filepath.Separator), "artifacts", "configs")
	out, err := RewriteConfigFilePaths([]byte(composeYAML), configsDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), filepath.Join(configsDir, "signing-key-pub")) {
		t.Errorf("expected rewritten path in output, got: %s", out)
	}
}

func TestWriteConfigs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "signing-key-pub")

	err := WriteConfigs(
		map[string]string{"signing-key-pub": path},
		map[string]string{"signing-key-pub": "-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----"},
	)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "BEGIN PUBLIC KEY") {
		t.Errorf("unexpected file content: %s", data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("expected mode 0644, got %v", info.Mode().Perm())
	}
}

func TestResolveConfigsPublicKey(t *testing.T) {
	privPEM, err := generatePrivateKey("ed25519", 0)
	if err != nil {
		t.Fatal(err)
	}

	configTemplates := map[string]Template{
		"signing-key-pub": {Type: TypePublicKey, Source: "signing-key"},
	}
	secretValues := map[string]string{"signing-key": privPEM}

	values, err := ResolveConfigs(configTemplates, secretValues)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(values["signing-key-pub"], "BEGIN PUBLIC KEY") {
		t.Errorf("expected derived public key PEM, got: %s", values["signing-key-pub"])
	}
}

func TestResolveConfigsMissingSource(t *testing.T) {
	configTemplates := map[string]Template{
		"signing-key-pub": {Type: TypePublicKey, Source: "signing-key"},
	}

	values, err := ResolveConfigs(configTemplates, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := values["signing-key-pub"]; ok {
		t.Error("expected no value when source secret is unresolved")
	}
}

func TestResolveConfigsNoSource(t *testing.T) {
	configTemplates := map[string]Template{
		"signing-key-pub": {Type: TypePublicKey},
	}

	_, err := ResolveConfigs(configTemplates, map[string]string{})
	if err == nil {
		t.Fatal("expected error for public_key config without source")
	}
}

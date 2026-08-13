package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigSetupFlow_DerivesPublicKeyFromSecret(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing-key.secret")
	pubPath := filepath.Join(dir, "signing-key-pub.config")

	composeYAML := `secrets:
  signing-key:
    file: ` + keyPath + `
configs:
  signing-key-pub:
    file: ` + pubPath + `
x-dargstack:
  secrets:
    signing-key:
      type: private_key
      key_type: ed25519
  configs:
    signing-key-pub:
      type: public_key
      source: signing-key
`

	noInteraction = true
	defer func() { noInteraction = false }()

	if _, err, _ := secretSetupFlow([]byte(composeYAML), false, false); err != nil {
		t.Fatalf("secretSetupFlow: %v", err)
	}

	issues, err := configSetupFlow([]byte(composeYAML))
	if err != nil {
		t.Fatalf("configSetupFlow: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}

	pub, err := os.ReadFile(pubPath)
	if err != nil {
		t.Fatalf("read derived public key: %v", err)
	}
	if !strings.Contains(string(pub), "BEGIN PUBLIC KEY") {
		t.Errorf("expected PEM-encoded public key, got: %s", pub)
	}

	info, err := os.Stat(pubPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("expected config file to be world-readable (0644), got %v", info.Mode().Perm())
	}
}

func TestConfigSetupFlow_SkipsWhenSourceUnresolved(t *testing.T) {
	dir := t.TempDir()
	pubPath := filepath.Join(dir, "signing-key-pub.config")

	composeYAML := `secrets:
  signing-key:
    file: ` + filepath.Join(dir, "signing-key.secret") + `
configs:
  signing-key-pub:
    file: ` + pubPath + `
x-dargstack:
  secrets:
    signing-key:
      type: private_key
      key_type: ed25519
  configs:
    signing-key-pub:
      type: public_key
      source: signing-key
`

	issues, err := configSetupFlow([]byte(composeYAML))
	if err != nil {
		t.Fatalf("configSetupFlow: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 warning issue, got %v", issues)
	}
	if issues[0].Resource != "config:signing-key-pub" {
		t.Errorf("unexpected issue resource: %s", issues[0].Resource)
	}

	if _, err := os.Stat(pubPath); !os.IsNotExist(err) {
		t.Errorf("expected no config file to be written, got err=%v", err)
	}
}

func TestConfigSetupFlow_NoConfigTemplates(t *testing.T) {
	issues, err := configSetupFlow([]byte(`services:
  api:
    image: api:latest
`))
	if err != nil {
		t.Fatalf("configSetupFlow: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

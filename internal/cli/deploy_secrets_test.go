package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dargstack/dargstack/v4/internal/secret"
)

func TestSecretSetupFlow_NoInteractionAutoGenerates(t *testing.T) {
	dir := t.TempDir()

	// Compose with auto-generatable secrets and a third_party secret.
	composeYAML := `secrets:
  api-key:
    file: ` + filepath.Join(dir, "api-key.secret") + `
  db-password:
    file: ` + filepath.Join(dir, "db-password.secret") + `
  external-token:
    file: ` + filepath.Join(dir, "external-token.secret") + `
x-dargstack:
  secrets:
    api-key:
      type: random_string
    db-password:
      type: wordlist_word
    external-token:
      type: third_party
`

	// Reset global state
	noInteraction = true
	defer func() { noInteraction = false }()

	err, _ := secretSetupFlow([]byte(composeYAML), false)
	if err != nil {
		t.Fatalf("secretSetupFlow: %v", err)
	}

	// Auto-generatable secrets should be written.
	apiKey, err := os.ReadFile(filepath.Join(dir, "api-key.secret"))
	if err != nil {
		t.Fatalf("read api-key: %v", err)
	}
	if strings.TrimSpace(string(apiKey)) == "" {
		t.Error("expected api-key to be auto-generated")
	}

	dbPass, err := os.ReadFile(filepath.Join(dir, "db-password.secret"))
	if err != nil {
		t.Fatalf("read db-password: %v", err)
	}
	if strings.TrimSpace(string(dbPass)) == "" {
		t.Error("expected db-password to be auto-generated")
	}

	// Third-party secret should have a placeholder.
	externalToken, err := os.ReadFile(filepath.Join(dir, "external-token.secret"))
	if err != nil {
		t.Fatalf("read external-token: %v", err)
	}
	if !secret.IsPlaceholderValue(strings.TrimSpace(string(externalToken))) {
		t.Errorf("expected external-token to have placeholder, got %q", string(externalToken))
	}
}

func TestSecretSetupFlow_NoInteractionTipOnlyForNoTemplate(t *testing.T) {
	dir := t.TempDir()

	// A secret with no x-dargstack.secrets definition.
	composeYAML := `secrets:
  api-key:
    file: ` + filepath.Join(dir, "api-key.secret") + `
  orphan-secret:
    file: ` + filepath.Join(dir, "orphan-secret.secret") + `
x-dargstack:
  secrets:
    api-key:
      type: random_string
`

	noInteraction = true
	defer func() { noInteraction = false }()

	err, _ := secretSetupFlow([]byte(composeYAML), false)
	if err != nil {
		t.Fatalf("secretSetupFlow: %v", err)
	}

	// api-key should be auto-generated.
	apiKey, err := os.ReadFile(filepath.Join(dir, "api-key.secret"))
	if err != nil {
		t.Fatalf("read api-key: %v", err)
	}
	if strings.TrimSpace(string(apiKey)) == "" {
		t.Error("expected api-key to be auto-generated")
	}

	// orphan-secret should NOT have been written (no template, no interactive prompt).
	if _, err := os.Stat(filepath.Join(dir, "orphan-secret.secret")); err == nil {
		t.Error("expected orphan-secret to not be created (no template, no interaction)")
	}
}

func TestSecretSetupFlow_AllSecretsSet(t *testing.T) {
	dir := t.TempDir()

	// All secrets are auto-generatable.
	composeYAML := `secrets:
  api-key:
    file: ` + filepath.Join(dir, "api-key.secret") + `
  db-password:
    file: ` + filepath.Join(dir, "db-password.secret") + `
x-dargstack:
  secrets:
    api-key:
      type: random_string
    db-password:
      type: insecure_default
      insecure_default: changeme
`

	noInteraction = true
	defer func() { noInteraction = false }()

	err, _ := secretSetupFlow([]byte(composeYAML), false)
	if err != nil {
		t.Fatalf("secretSetupFlow: %v", err)
	}

	// Both should be written.
	apiKey, err := os.ReadFile(filepath.Join(dir, "api-key.secret"))
	if err != nil {
		t.Fatalf("read api-key: %v", err)
	}
	if strings.TrimSpace(string(apiKey)) == "" {
		t.Error("expected api-key to be auto-generated")
	}

	dbPass, err := os.ReadFile(filepath.Join(dir, "db-password.secret"))
	if err != nil {
		t.Fatalf("read db-password: %v", err)
	}
	if strings.TrimSpace(string(dbPass)) != "changeme" {
		t.Errorf("expected db-password=changeme, got %q", string(dbPass))
	}
}

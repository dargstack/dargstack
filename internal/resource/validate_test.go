package resource

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateMissingSecrets(t *testing.T) {
	stackDir := t.TempDir()
	secretsDir := filepath.Join(stackDir, "src", "development", "api")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	secretFile := filepath.Join(secretsDir, "db-password.secret")
	if err := os.WriteFile(secretFile, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	missingFile := filepath.Join(secretsDir, "missing.secret")

	// After merge, file paths are absolute.
	composeYAML := `services:
  api:
    image: api:latest
secrets:
  api-db-password:
    file: ` + secretFile + `
  api-missing:
    file: ` + missingFile + `
`
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	foundMissing := false
	for _, iss := range issues {
		if iss.Severity == "error" && iss.Resource == "secret:api-missing" {
			foundMissing = true
		}
	}
	if !foundMissing {
		t.Error("expected error for missing secret api-missing")
	}
}

func TestValidateMissingThirdPartySecretIsWarning(t *testing.T) {
	stackDir := t.TempDir()
	missingFile := filepath.Join(stackDir, "src", "development", "api", "thirdparty.secret")

	composeYAML := `services:
  api:
    image: api:latest
secrets:
  api-thirdparty:
    file: ` + missingFile + `
x-dargstack:
  secrets:
    api-thirdparty:
      third_party: true
`

	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	foundWarning := false
	foundError := false
	for _, iss := range issues {
		if iss.Resource != "secret:api-thirdparty" {
			continue
		}
		if iss.Severity == "warning" {
			foundWarning = true
		}
		if iss.Severity == "error" {
			foundError = true
		}
	}

	if !foundWarning {
		t.Error("expected warning for missing third_party secret")
	}
	if foundError {
		t.Error("expected no error for missing third_party secret")
	}
}

func TestValidateCertificates(t *testing.T) {
	stackDir := t.TempDir()

	composeYAML := `services:
  api:
    image: api:latest
`
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	foundCertWarning := false
	for _, iss := range issues {
		if iss.Resource == "certificates" {
			foundCertWarning = true
		}
	}
	if !foundCertWarning {
		t.Error("expected warning about missing certificates directory")
	}
}

func TestMissingSecrets(t *testing.T) {
	issues := []Issue{
		{Severity: "error", Resource: "secret:api-db", Description: "not found"},
		{Severity: "warning", Resource: "certificates", Description: "missing"},
		{Severity: "error", Resource: "secret:api-key", Description: "not found"},
	}

	missing := MissingSecrets(issues)
	if len(missing) != 2 {
		t.Errorf("expected 2 missing secrets, got %d", len(missing))
	}
}

func TestGenerateDocumentation(t *testing.T) {
	// Set up a temporary stack directory with service compose files.
	stackDir := t.TempDir()
	devDir := filepath.Join(stackDir, "src", "development")
	prodDir := filepath.Join(stackDir, "src", "production")

	// Dev service: api (with a YAML comment)
	apiDir := filepath.Join(devDir, "api")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	apiCompose := `services:
  api:
    # The main API service.
    # Handles HTTP requests.
    image: api:latest
    ports:
      - "3000:3000"
    secrets:
      - api-db
`
	if err := os.WriteFile(filepath.Join(apiDir, "compose.yaml"), []byte(apiCompose), 0o644); err != nil {
		t.Fatal(err)
	}

	// Dev service: postgres (no comment) — also defines a "redis" service
	pgDir := filepath.Join(devDir, "postgres")
	if err := os.MkdirAll(pgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pgCompose := `services:
  postgres:
    image: postgres:16
    volumes:
      - postgres-data:/var/lib/postgresql/data
  redis:
    # In-memory cache.
    image: redis:7
`
	if err := os.WriteFile(filepath.Join(pgDir, "compose.yaml"), []byte(pgCompose), 0o644); err != nil {
		t.Fatal(err)
	}

	// Prod-only service: worker
	workerDir := filepath.Join(prodDir, "worker")
	if err := os.MkdirAll(workerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workerCompose := `services:
  worker:
    # Background job processor.
    image: worker:latest
`
	if err := os.WriteFile(filepath.Join(workerDir, "compose.yaml"), []byte(workerCompose), 0o644); err != nil {
		t.Fatal(err)
	}

	content, err := GenerateDocumentation(&DocsConfig{
		StackDir:       stackDir,
		StackName:      "example-stack",
		StackDomain:    "example.localhost",
		SourceCodeName: "my-project",
		SourceCodeURL:  "https://github.com/example/my-project",
	})
	if err != nil {
		t.Fatal(err)
	}

	expected := `# example-stack

The Docker stack configuration for [example.localhost](https://example.localhost/). Related to [my-project](https://github.com/example/my-project).

## Services

### api

The main API service.
Handles HTTP requests.

### postgres

### redis

In-memory cache.

### worker *(production only)*

Background job processor.

`
	if content != expected {
		t.Fatalf("documentation mismatch:\n--- got ---\n%s\n--- expected ---\n%s", content, expected)
	}
}

func TestValidateConfigs(t *testing.T) {
	stackDir := t.TempDir()

	// Create a real config file
	cfgFile := filepath.Join(stackDir, "nginx.conf")
	if err := os.WriteFile(cfgFile, []byte("worker_processes auto;"), 0o644); err != nil {
		t.Fatal(err)
	}

	missingFile := filepath.Join(stackDir, "missing.conf")

	composeYAML := `services:
  web:
    image: nginx:latest
configs:
  nginx.conf:
    file: ` + cfgFile + `
  missing.conf:
    file: ` + missingFile + `
  external.conf:
    external: true
`
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	var configErrors []string
	for _, iss := range issues {
		if len(iss.Resource) >= 7 && iss.Resource[:7] == "config:" && iss.Severity == "error" {
			configErrors = append(configErrors, iss.Resource)
		}
	}

	if len(configErrors) != 1 {
		t.Fatalf("expected 1 config error, got %d: %v", len(configErrors), configErrors)
	}
	if configErrors[0] != "config:missing.conf" {
		t.Errorf("expected error for missing.conf, got %s", configErrors[0])
	}
}

func TestExtractDargstackBuildLabelMapForm(t *testing.T) {
	svc := map[string]interface{}{
		"image": "myapp:latest",
		"deploy": map[string]interface{}{
			"labels": map[string]interface{}{
				"dargstack.development.build": "./context",
			},
		},
	}
	got := extractDargstackBuildLabel(svc)
	if got != "./context" {
		t.Errorf("expected ./context, got %q", got)
	}
}

func TestExtractDargstackBuildLabelListForm(t *testing.T) {
	svc := map[string]interface{}{
		"image": "myapp:latest",
		"deploy": map[string]interface{}{
			"labels": []interface{}{
				"dargstack.development.build=./myapp",
				"other.label=value",
			},
		},
	}
	got := extractDargstackBuildLabel(svc)
	if got != "./myapp" {
		t.Errorf("expected ./myapp, got %q", got)
	}
}

func TestExtractDargstackBuildLabelMissing(t *testing.T) {
	svc := map[string]interface{}{
		"image": "myapp:latest",
	}
	got := extractDargstackBuildLabel(svc)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestValidateServicesMissingDockerfile(t *testing.T) {
	stackDir := t.TempDir()

	composeYAML := `services:
  myapp:
    image: myapp:latest
    deploy:
      labels:
        dargstack.development.build: ./context
`
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	foundErr := false
	for _, iss := range issues {
		if iss.Resource == "service:myapp" && iss.Severity == "error" {
			foundErr = true
		}
	}
	if !foundErr {
		t.Error("expected error for service with missing Dockerfile")
	}
}

func TestValidateThirdPartySecretPlaceholderContent(t *testing.T) {
	stackDir := t.TempDir()
	secretsDir := filepath.Join(stackDir, "src", "development", "api")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	secretFile := filepath.Join(secretsDir, "thirdparty.secret")
	if err := os.WriteFile(secretFile, []byte("UNSET THIRD PARTY SECRET\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	composeYAML := `services:
  api:
    image: api:latest
secrets:
  api-thirdparty:
    file: ` + secretFile + `
x-dargstack:
  secrets:
    api-thirdparty:
      third_party: true
`

	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	foundWarning := false
	for _, iss := range issues {
		if iss.Resource == "secret:api-thirdparty" && iss.Severity == "warning" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Error("expected warning for third_party secret with placeholder content")
	}
}

func TestValidateThirdPartySecretRealContentNoWarning(t *testing.T) {
	stackDir := t.TempDir()
	secretsDir := filepath.Join(stackDir, "src", "development", "api")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	secretFile := filepath.Join(secretsDir, "thirdparty.secret")
	if err := os.WriteFile(secretFile, []byte("my-real-api-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	composeYAML := `services:
  api:
    image: api:latest
secrets:
  api-thirdparty:
    file: ` + secretFile + `
x-dargstack:
  secrets:
    api-thirdparty:
      third_party: true
`

	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	for _, iss := range issues {
		if iss.Resource == "secret:api-thirdparty" {
			t.Errorf("expected no issue for third_party secret with real content, got: %s", iss.Description)
		}
	}
}

func TestValidateServicesWithDockerfile(t *testing.T) {
	stackDir := t.TempDir()

	// Create relative context path inside src/development/myapp
	contextDir := filepath.Join(stackDir, "src", "development", "myapp", "context")
	if err := os.MkdirAll(contextDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contextDir, "Dockerfile"), []byte("FROM scratch"), 0o644); err != nil {
		t.Fatal(err)
	}

	composeYAML := `services:
  myapp:
    image: myapp:latest
    deploy:
      labels:
        dargstack.development.build: ./context
`
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	for _, iss := range issues {
		if iss.Resource == "service:myapp" && iss.Severity == "error" {
			t.Errorf("unexpected error for service with Dockerfile: %s", iss.Description)
		}
	}
}

func TestValidateGitLabelUnclonedWarning(t *testing.T) {
	stackDir := t.TempDir()

	composeYAML := `services:
  webapp:
    image: webapp:latest
    deploy:
      labels:
        dargstack.development.git: "git@github.com:organization/repository.git"
`
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	foundWarning := false
	for _, iss := range issues {
		if iss.Resource == "service:webapp" && iss.Severity == "warning" {
			foundWarning = true
			if !strings.Contains(iss.Description, "will be cloned") {
				t.Errorf("expected warning about cloning, got: %s", iss.Description)
			}
		}
	}
	if !foundWarning {
		t.Error("expected warning for uncloned git repo")
	}
}

func TestValidateGitLabelNoWarningWhenCloned(t *testing.T) {
	stackDir := t.TempDir()

	// Create the cloned directory to simulate already cloned
	// Implementation uses filepath.Dir(stackDir) as parent for clones
	clonedDir := filepath.Join(filepath.Dir(stackDir), "repository")
	if err := os.MkdirAll(clonedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	composeYAML := `services:
  webapp:
    image: webapp:latest
    deploy:
      labels:
        dargstack.development.git: "git@github.com:organization/repository.git"
`
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	for _, iss := range issues {
		if iss.Resource == "service:webapp" && iss.Severity == "warning" && strings.Contains(iss.Description, "cloned") {
			t.Errorf("expected no clone warning when repo exists, got: %s", iss.Description)
		}
	}
}

func TestValidateGitLabelSkippedInProduction(t *testing.T) {
	stackDir := t.TempDir()

	composeYAML := `services:
  webapp:
    image: webapp:latest
    deploy:
      labels:
        dargstack.development.git: "git@github.com:organization/repository.git"
`
	issues, err := Validate([]byte(composeYAML), stackDir, true)
	if err != nil {
		t.Fatal(err)
	}

	for _, iss := range issues {
		if iss.Resource == "service:webapp" && strings.Contains(iss.Description, "cloned") {
			t.Error("expected no git warning in production mode")
		}
	}
}

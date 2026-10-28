package resource

import (
	"crypto/sha256"
	"encoding/hex"
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

	// Dev service: postgres (no comment), also defines a "redis" service
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
		DevDir:         devDir,
		ProdDir:        prodDir,
		SourceCodeName: "example",
		SourceCodeURL:  "https://github.com/example",
		StackDomain:    "example.localhost",
		StackName:      "example-stack",
	})
	if err != nil {
		t.Fatal(err)
	}

	expected := `# example-stack

The Docker stack configuration for [example.localhost](https://example.localhost/). See [example](https://github.com/example).

## Services

Each service corresponds to a compose.yaml file. Descriptions are extracted from YAML comments in the source. Services marked *(production only)* exist only in the production overlay.

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

func TestValidateDargstackConfigsPublicKey(t *testing.T) {
	stackDir := t.TempDir()

	keyFile := filepath.Join(stackDir, "signing-key")
	if err := os.WriteFile(keyFile, []byte("-----BEGIN PRIVATE KEY-----\n-----END PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pubFile := filepath.Join(stackDir, "signing-key-pub")
	if err := os.WriteFile(pubFile, []byte("-----BEGIN PUBLIC KEY-----\n-----END PUBLIC KEY-----\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	composeYAML := `services:
  api:
    image: api:latest
x-dargstack:
  secrets:
    signing-key:
      type: private_key
      key_type: ed25519
  configs:
    signing-key-pub:
      type: public_key
      source: signing-key
secrets:
  signing-key:
    file: ` + keyFile + `
configs:
  signing-key-pub:
    file: ` + pubFile + `
`

	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, iss := range issues {
		if iss.Severity == "error" {
			t.Errorf("unexpected error: %s", iss)
		}
	}
}

func TestValidateDargstackConfigsPublicKeyTrimsSourceWhitespace(t *testing.T) {
	stackDir := t.TempDir()

	keyFile := filepath.Join(stackDir, "signing-key")
	if err := os.WriteFile(keyFile, []byte("-----BEGIN PRIVATE KEY-----\n-----END PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pubFile := filepath.Join(stackDir, "signing-key-pub")
	if err := os.WriteFile(pubFile, []byte("-----BEGIN PUBLIC KEY-----\n-----END PUBLIC KEY-----\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	composeYAML := `services:
  api:
    image: api:latest
x-dargstack:
  secrets:
    signing-key:
      type: private_key
      key_type: ed25519
  configs:
    signing-key-pub:
      type: public_key
      source: " signing-key "
secrets:
  signing-key:
    file: ` + keyFile + `
configs:
  signing-key-pub:
    file: ` + pubFile + `
`

	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, iss := range issues {
		if iss.Severity == "error" {
			t.Errorf("unexpected error for source with surrounding whitespace: %s", iss)
		}
	}
}

func TestValidateDargstackConfigsUnknownSource(t *testing.T) {
	composeYAML := `services:
  api:
    image: api:latest
x-dargstack:
  configs:
    signing-key-pub:
      type: public_key
      source: does-not-exist
`
	stackDir := t.TempDir()
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, iss := range issues {
		if iss.Resource == "config:signing-key-pub" && iss.Severity == "error" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for config referencing unknown source, got: %v", issues)
	}
}

func TestValidateDargstackConfigsSourceNotPrivateKey(t *testing.T) {
	composeYAML := `services:
  api:
    image: api:latest
x-dargstack:
  secrets:
    api-token:
      length: 32
  configs:
    signing-key-pub:
      type: public_key
      source: api-token
`
	stackDir := t.TempDir()
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, iss := range issues {
		if iss.Resource == "config:signing-key-pub" && iss.Severity == "error" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for config sourcing a non-private_key secret, got: %v", issues)
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

func TestValidateDargstackSecretsBasicAuth(t *testing.T) {
	cases := []struct {
		name        string
		definition  string
		wantIssue   bool
		wantMessage string
	}{
		{
			name: "valid",
			definition: `    admin-auth:
      type: basic_auth
      username: admin
      source: admin-password
    admin-password:
      type: random_string
`,
			wantIssue: false,
		},
		{
			name: "missing source",
			definition: `    admin-auth:
      type: basic_auth
      username: admin
`,
			wantIssue:   true,
			wantMessage: "requires a source",
		},
		{
			name: "unknown source",
			definition: `    admin-auth:
      type: basic_auth
      username: admin
      source: does-not-exist
`,
			wantIssue:   true,
			wantMessage: "not defined in x-dargstack.secrets",
		},
		{
			name: "missing username",
			definition: `    admin-auth:
      type: basic_auth
      source: admin-password
    admin-password:
      type: random_string
`,
			wantIssue:   true,
			wantMessage: "requires a username",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			composeYAML := `services:
  api:
    image: api:latest
x-dargstack:
  secrets:
` + c.definition

			issues, err := Validate([]byte(composeYAML), t.TempDir(), false)
			if err != nil {
				t.Fatal(err)
			}

			found := false
			for _, iss := range issues {
				if iss.Resource == "secret:admin-auth" && iss.Severity == "error" {
					found = true
					if c.wantMessage != "" && !strings.Contains(iss.Description, c.wantMessage) {
						t.Errorf("description = %q, want it to contain %q", iss.Description, c.wantMessage)
					}
				}
			}
			if found != c.wantIssue {
				t.Errorf("got issue = %v, want %v (issues: %v)", found, c.wantIssue, issues)
			}
		})
	}
}

func TestValidateDargstackSecretsBasicAuthPasswordTooLongForBcrypt(t *testing.T) {
	composeYAML := `services:
  api:
    image: api:latest
x-dargstack:
  secrets:
    admin-auth:
      type: basic_auth
      username: admin
      source: admin-password
    admin-password:
      type: random_string
      length: 100
`
	issues, err := Validate([]byte(composeYAML), t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, iss := range issues {
		if iss.Resource == "secret:admin-auth" && iss.Severity == "error" && strings.Contains(iss.Description, "bcrypt cannot hash") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an error about the bcrypt length limit, got: %v", issues)
	}
}

func TestValidateConfigHashSuffixMismatch(t *testing.T) {
	stackDir := t.TempDir()
	cfgFile := filepath.Join(stackDir, "prometheus.yaml")
	if err := os.WriteFile(cfgFile, []byte("global:\n  scrape_interval: 30s\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	composeYAML := `configs:
  prometheus-yaml-a1b2c3d4:
    file: ` + cfgFile + `
services:
  prometheus:
    image: prom/prometheus:latest
    configs: [prometheus-yaml-a1b2c3d4]
`
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, iss := range issues {
		if iss.Severity != "error" || iss.Resource != "config:prometheus-yaml-a1b2c3d4" {
			continue
		}
		found = true
		if !strings.Contains(iss.Description, "content hash mismatch") {
			t.Errorf("unexpected description: %s", iss.Description)
		}
		if !strings.Contains(iss.Description, "name has a1b2c3d4") {
			t.Errorf("expected old hash in message: %s", iss.Description)
		}
		if !strings.Contains(iss.Description, "dargstack config rotate prometheus-yaml-a1b2c3d4") {
			t.Errorf("expected rotate hint: %s", iss.Description)
		}
	}
	if !found {
		t.Fatalf("expected hash mismatch error, got %v", issues)
	}
}

func TestValidateConfigHashSuffixMatch(t *testing.T) {
	stackDir := t.TempDir()
	content := []byte("global:\n  scrape_interval: 30s\n")
	cfgFile := filepath.Join(stackDir, "prometheus.yaml")
	if err := os.WriteFile(cfgFile, content, 0o644); err != nil {
		t.Fatal(err)
	}
	hash := ""
	{
		sum := sha256.Sum256(content)
		hash = hex.EncodeToString(sum[:])[:8]
	}

	composeYAML := "configs:\n  prometheus-yaml-" + hash + ":\n    file: " + cfgFile + "\nservices:\n  prometheus:\n    image: prom/prometheus:latest\n    configs: [prometheus-yaml-" + hash + "]\n"
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, iss := range issues {
		if strings.Contains(iss.Description, "content hash mismatch") {
			t.Fatalf("expected no hash mismatch, got %v", issues)
		}
	}
}

func TestValidateConfigWithoutSuffixNotPoliced(t *testing.T) {
	stackDir := t.TempDir()
	cfgFile := filepath.Join(stackDir, "prometheus.yaml")
	if err := os.WriteFile(cfgFile, []byte("anything\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	composeYAML := `configs:
  prometheus-yaml:
    file: ` + cfgFile + `
`
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, iss := range issues {
		if strings.Contains(iss.Description, "content hash mismatch") {
			t.Fatalf("names without a suffix must not be policed, got %v", issues)
		}
	}
}

func TestValidateConfigHashSuffixMissingFile(t *testing.T) {
	stackDir := t.TempDir()

	composeYAML := `configs:
  prometheus-yaml-a1b2c3d4:
    file: ` + filepath.Join(stackDir, "missing.yaml") + `
`
	issues, err := Validate([]byte(composeYAML), stackDir, false)
	if err != nil {
		t.Fatal(err)
	}
	var missing, mismatch int
	for _, iss := range issues {
		if iss.Severity != "error" || iss.Resource != "config:prometheus-yaml-a1b2c3d4" {
			continue
		}
		switch {
		case strings.Contains(iss.Description, "file not found"):
			missing++
		case strings.Contains(iss.Description, "content hash mismatch"):
			mismatch++
		}
	}
	if missing != 1 {
		t.Errorf("expected exactly one file-not-found error, got %v", issues)
	}
	if mismatch != 0 {
		t.Errorf("no hash check possible for a missing file, got %v", issues)
	}
}

func TestValidateServiceReferencesStale(t *testing.T) {
	// A reference inherited from a shared definition (anchor/merge key) is explicit in the merged document,
	// because the YAML merge resolves it before this check runs. A stale one must be flagged even though the
	// top-level section only carries the rotated name, and the same applies to secrets.
	composeYAML := `services:
  one:
    image: example/app:1.0.0
    configs:
      - source: app-config
        target: /etc/app/config.yaml
    secrets: [app-secret]
  two:
    image: example/app:1.0.0
    configs: [other-config]
    secrets:
      - source: other-secret
        target: /etc/other/secret
configs:
  app-config-b2c3d4e5:
    file: ./config.yaml
  other-config:
    file: ./other.yaml
secrets:
  other-secret:
    file: ./other.secret
`
	issues, err := ValidateServiceReferences([]byte(composeYAML))
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 {
		t.Fatalf("expected exactly 2 issues, got %v", issues)
	}
	foundConfig, foundSecret := false, false
	for _, iss := range issues {
		if iss.Severity != "error" {
			t.Errorf("expected error severity, got %s", iss.Severity)
		}
		if iss.Resource != "service:one" {
			t.Errorf("expected resource service:one, got %s", iss.Resource)
		}
		switch {
		case strings.Contains(iss.Description, `"app-config"`):
			foundConfig = true
		case strings.Contains(iss.Description, `"app-secret"`):
			foundSecret = true
		default:
			t.Errorf("unexpected description: %s", iss.Description)
		}
	}
	if !foundConfig || !foundSecret {
		t.Errorf("expected a stale config and a stale secret, got %v", issues)
	}
}

func TestValidateServiceReferencesInterpolatedSkipped(t *testing.T) {
	// Compose interpolation resolves against the deployment environment, which dargstack does not see; such
	// references are left to `docker compose config` and must not be reported as undefined.
	composeYAML := `services:
  one:
    image: example/app:1.0.0
    configs:
      - "${CONFIG_NAME:-app-config}"
    secrets:
      - source: "${SECRET_NAME}"
        target: /etc/secret
configs:
  app-config:
    file: ./config.yaml
secrets:
  app-secret:
    file: ./app.secret
`
	issues, err := ValidateServiceReferences([]byte(composeYAML))
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues for interpolated references, got %v", issues)
	}
}

func TestValidateServiceReferencesAllDefined(t *testing.T) {
	composeYAML := `services:
  one:
    image: example/app:1.0.0
    configs: [app-config]
    secrets: [app-secret]
  two:
    image: example/app:1.0.0
    configs:
      - source: other-config
        target: /etc/other.yaml
    secrets:
      - source: other-secret
        target: /etc/other/secret
configs:
  app-config:
    file: ./config.yaml
  other-config:
    file: ./other.yaml
secrets:
  app-secret:
    file: ./app.secret
  other-secret:
    file: ./other.secret
`
	issues, err := ValidateServiceReferences([]byte(composeYAML))
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

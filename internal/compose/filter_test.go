package compose

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFilterByProfile(t *testing.T) {
	composeYAML := `services:
  api:
    image: myapi:latest
    secrets:
      - api-db-password
    volumes:
      - api-data:/data
  postgres:
    image: postgres:16
    volumes:
      - postgres-data:/var/lib/postgresql/data
    secrets:
      - postgres-password
  debug:
    image: debug:latest
    deploy:
      labels:
        dargstack.profiles: debug
  monitoring:
    image: grafana:latest
    deploy:
      labels:
        dargstack.profiles: monitoring
    volumes:
      - monitoring-data:/data
secrets:
  api-db-password:
    file: ./secrets/api-db-password.secret
  postgres-password:
    file: ./secrets/postgres-password.secret
volumes:
  api-data:
  postgres-data:
  monitoring-data:
`

	// No profile and no "default" profile exists — all services deployed
	result, err := FilterByProfile([]byte(composeYAML), nil)
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatal(err)
	}

	services := doc["services"].(map[string]interface{})
	if _, ok := services["api"]; !ok {
		t.Error("expected api service to be present")
	}
	if _, ok := services["postgres"]; !ok {
		t.Error("expected postgres service to be present")
	}
	if _, ok := services["debug"]; !ok {
		t.Error("expected debug service to be present (no default profile → deploy all)")
	}
	if _, ok := services["monitoring"]; !ok {
		t.Error("expected monitoring service to be present (no default profile → deploy all)")
	}
}

func TestFilterByProfileDefault(t *testing.T) {
	composeYAML := `services:
  api:
    image: myapi:latest
  postgres:
    image: postgres:16
    deploy:
      labels:
        dargstack.profiles: default
  debug:
    image: debug:latest
    deploy:
      labels:
        dargstack.profiles: debug
  monitoring:
    image: grafana:latest
    deploy:
      labels:
        dargstack.profiles: "default,monitoring"
`

	// No explicit profile — auto-activates "default"
	result, err := FilterByProfile([]byte(composeYAML), nil)
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatal(err)
	}

	services := doc["services"].(map[string]interface{})
	if _, ok := services["api"]; ok {
		t.Error("expected api (no profiles) to be absent when default profile exists")
	}
	if _, ok := services["postgres"]; !ok {
		t.Error("expected postgres (default profile) to be present")
	}
	if _, ok := services["debug"]; ok {
		t.Error("expected debug (non-default profile) to be absent")
	}
	if _, ok := services["monitoring"]; !ok {
		t.Error("expected monitoring (includes default profile) to be present")
	}
}

func TestFilterByProfileActivated(t *testing.T) {
	composeYAML := `services:
  api:
    image: myapi:latest
  debug:
    image: debug:latest
    deploy:
      labels:
        dargstack.profiles: debug
  monitoring:
    image: grafana:latest
    deploy:
      labels:
        dargstack.profiles: monitoring
`

	result, err := FilterByProfile([]byte(composeYAML), []string{"debug"})
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatal(err)
	}

	services := doc["services"].(map[string]interface{})
	if _, ok := services["api"]; !ok {
		t.Error("expected api (no profiles) to be present")
	}
	if _, ok := services["debug"]; !ok {
		t.Error("expected debug (profile activated) to be present")
	}
	if _, ok := services["monitoring"]; ok {
		t.Error("expected monitoring to be absent (different profile)")
	}
}

func TestFilterByProfileActivatedWithDefaultExcludesUnlabeled(t *testing.T) {
	composeYAML := `services:
  api:
    image: myapi:latest
  postgres:
    image: postgres:16
    deploy:
      labels:
        dargstack.profiles: default
  debug:
    image: debug:latest
    deploy:
      labels:
        dargstack.profiles: debug
`

	result, err := FilterByProfile([]byte(composeYAML), []string{"debug"})
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatal(err)
	}

	services := doc["services"].(map[string]interface{})
	if _, ok := services["api"]; ok {
		t.Error("expected api (unlabeled) to be absent when default profile exists and unlabeled is not active")
	}
	if _, ok := services["debug"]; !ok {
		t.Error("expected debug to be present")
	}
}

func TestFilterByProfileActivatedWithDefaultAndUnlabeledIncludesUnlabeled(t *testing.T) {
	composeYAML := `services:
  api:
    image: myapi:latest
  postgres:
    image: postgres:16
    deploy:
      labels:
        dargstack.profiles: default
  debug:
    image: debug:latest
    deploy:
      labels:
        dargstack.profiles: debug
`

	result, err := FilterByProfile([]byte(composeYAML), []string{"debug", "unlabeled"})
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatal(err)
	}

	services := doc["services"].(map[string]interface{})
	if _, ok := services["api"]; !ok {
		t.Error("expected api (unlabeled) to be present when unlabeled profile is active")
	}
	if _, ok := services["debug"]; !ok {
		t.Error("expected debug to be present")
	}
}

func TestFilterServices(t *testing.T) {
	composeYAML := `services:
  api:
    image: myapi:latest
    secrets:
      - api-db-password
    volumes:
      - api-data:/data
  postgres:
    image: postgres:16
    volumes:
      - postgres-data:/var/lib/postgresql/data
    secrets:
      - postgres-password
  redis:
    image: redis:7
secrets:
  api-db-password:
    file: ./secrets/api-db-password.secret
  postgres-password:
    file: ./secrets/postgres-password.secret
volumes:
  api-data:
  postgres-data:
  redis-data:
`

	result, err := FilterServices([]byte(composeYAML), []string{"api"})
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatal(err)
	}

	services := doc["services"].(map[string]interface{})
	if _, ok := services["api"]; !ok {
		t.Error("expected api service to be present")
	}
	if _, ok := services["postgres"]; ok {
		t.Error("expected postgres service to be absent")
	}
	if _, ok := services["redis"]; ok {
		t.Error("expected redis service to be absent")
	}

	secrets := doc["secrets"].(map[string]interface{})
	if _, ok := secrets["postgres-password"]; ok {
		t.Error("expected postgres-password to be removed")
	}

	volumes := doc["volumes"].(map[string]interface{})
	if _, ok := volumes["redis-data"]; ok {
		t.Error("expected redis-data volume to be removed")
	}
}

func TestDiscoverProfiles(t *testing.T) {
	composeYAML := `services:
  api:
    image: api:latest
  debug:
    image: debug:latest
    deploy:
      labels:
        dargstack.profiles: debug
  monitoring:
    image: grafana:latest
    deploy:
      labels:
        dargstack.profiles: "monitoring,debug"
`

	profiles, err := DiscoverProfiles([]byte(composeYAML))
	if err != nil {
		t.Fatal(err)
	}

	profileSet := make(map[string]bool)
	for _, p := range profiles {
		profileSet[p] = true
	}

	if !profileSet["debug"] {
		t.Error("expected debug profile to be discovered")
	}
	if !profileSet["monitoring"] {
		t.Error("expected monitoring profile to be discovered")
	}
}

func TestExtractVolumeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"myvolume:/data", "myvolume"},
		{"/host/path:/data", ""},
		{"./relative:/data", ""},
		{"named_vol:/app/data:ro", "named_vol"},
		// Windows drive letters should be treated as bind mounts (return "")
		{`C:\data:/container`, ""},
		{`C:/data:/container`, ""},
		// A bare drive letter without a path separator is ambiguous but rare;
		// treat as bind mount to be safe.
		{"C:/container", ""},
	}

	for _, tt := range tests {
		got := extractVolumeName(tt.input)
		if got != tt.expected {
			t.Errorf("extractVolumeName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFilterByProfileStripsOutOfProfileDargstackSecrets(t *testing.T) {
	composeYAML := `services:
  api:
    image: api:latest
    secrets:
      - api
    deploy:
      labels:
        dargstack.profiles: default
  grafana:
    image: grafana:latest
    secrets:
      - grafana-webhook
    deploy:
      labels:
        dargstack.profiles: monitoring
secrets:
  api:
    file: ./secrets/api.secret
  grafana-webhook:
    file: ./secrets/grafana-webhook.secret
x-dargstack:
  secrets:
    api:
      type: random_string
    grafana-webhook:
      type: third_party
      hint: "https://discord.com/api/webhooks/..."
`

	result, err := FilterByProfile([]byte(composeYAML), []string{"default"})
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatal(err)
	}

	ext, ok := doc["x-dargstack"].(map[string]interface{})
	if !ok {
		t.Fatal("expected x-dargstack extension to be present")
	}
	secrets, ok := ext["secrets"].(map[string]interface{})
	if !ok {
		t.Fatal("expected x-dargstack.secrets to be present")
	}
	if _, ok := secrets["api"]; !ok {
		t.Error("expected api secret template to be retained for active profile")
	}
	if _, ok := secrets["grafana-webhook"]; ok {
		t.Error("expected grafana-webhook secret template to be removed for inactive profile")
	}
}

func TestFilterByProfileLongFormSecretsKept(t *testing.T) {
	// Long-form secret reference: secrets: [{ source: name, target: /run/... }]
	composeYAML := `services:
  api:
    image: api:latest
    secrets:
      - source: api-db
        target: /run/secrets/db
    deploy:
      labels:
        dargstack.profiles: default
  other:
    image: other:latest
    deploy:
      labels:
        dargstack.profiles: other
secrets:
  api-db:
    file: ./secrets/api.db.secret
  other:
    file: ./secrets/other.secret
`
	result, err := FilterByProfile([]byte(composeYAML), []string{"default"})
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatal(err)
	}

	secrets, ok := doc["secrets"].(map[string]interface{})
	if !ok {
		t.Fatal("expected secrets section")
	}
	if _, ok := secrets["api-db"]; !ok {
		t.Error("expected api-db secret to be kept (referenced via long form)")
	}
	if _, ok := secrets["other"]; ok {
		t.Error("expected other secret to be removed (unreferenced after profile filter)")
	}
}

func TestFilterByProfileKeepsTransitiveSecretDeps(t *testing.T) {
	// A service references "aws-credentials" directly.
	// The "aws-credentials" template references "aws-access-key" and "aws-secret-key"
	// via {{secret:...}}. Those are NOT directly referenced by any service.
	// They should still be kept in both secrets: and x-dargstack.secrets.
	composeYAML := `services:
  app:
    image: app:latest
    secrets:
      - aws-credentials
    deploy:
      labels:
        dargstack.profiles: default
  monitoring:
    image: grafana:latest
    secrets:
      - grafana-webhook
    deploy:
      labels:
        dargstack.profiles: monitoring
secrets:
  aws-credentials:
    file: ./secrets/aws-credentials.secret
  aws-access-key:
    file: ./secrets/aws-access-key.secret
  aws-secret-key:
    file: ./secrets/aws-secret-key.secret
  grafana-webhook:
    file: ./secrets/grafana-webhook.secret
x-dargstack:
  secrets:
    aws-credentials:
      template: |
        [default]
        aws_access_key_id = {{secret:aws-access-key}}
        aws_secret_access_key = {{secret:aws-secret-key}}
    aws-access-key:
      type: third_party
    aws-secret-key:
      type: third_party
    grafana-webhook:
      type: third_party
      hint: "https://discord.com/api/webhooks/..."
`

	result, err := FilterByProfile([]byte(composeYAML), []string{"default"})
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatal(err)
	}

	// Check top-level secrets: section
	topSecrets, ok := doc["secrets"].(map[string]interface{})
	if !ok {
		t.Fatal("expected secrets section")
	}
	if _, ok := topSecrets["aws-credentials"]; !ok {
		t.Error("expected aws-credentials in secrets (directly referenced)")
	}
	if _, ok := topSecrets["aws-access-key"]; !ok {
		t.Error("expected aws-access-key in secrets (transitively referenced by aws-credentials template)")
	}
	if _, ok := topSecrets["aws-secret-key"]; !ok {
		t.Error("expected aws-secret-key in secrets (transitively referenced by aws-credentials template)")
	}
	if _, ok := topSecrets["grafana-webhook"]; ok {
		t.Error("expected grafana-webhook to be removed (inactive profile)")
	}

	// Check x-dargstack.secrets section
	ext, ok := doc["x-dargstack"].(map[string]interface{})
	if !ok {
		t.Fatal("expected x-dargstack extension")
	}
	dargSecrets, ok := ext["secrets"].(map[string]interface{})
	if !ok {
		t.Fatal("expected x-dargstack.secrets")
	}
	if _, ok := dargSecrets["aws-credentials"]; !ok {
		t.Error("expected aws-credentials in x-dargstack.secrets")
	}
	if _, ok := dargSecrets["aws-access-key"]; !ok {
		t.Error("expected aws-access-key in x-dargstack.secrets (transitive dependency)")
	}
	if _, ok := dargSecrets["aws-secret-key"]; !ok {
		t.Error("expected aws-secret-key in x-dargstack.secrets (transitive dependency)")
	}
	if _, ok := dargSecrets["grafana-webhook"]; ok {
		t.Error("expected grafana-webhook to be removed from x-dargstack.secrets (inactive profile)")
	}
}

func TestFilterServicesByName(t *testing.T) {
	composeYAML := `services:
  api:
    image: api:latest
  postgres:
    image: postgres:16
  redis:
    image: redis:7
`
	result, err := FilterServices([]byte(composeYAML), []string{"api", "redis"})
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatal(err)
	}

	services := doc["services"].(map[string]interface{})
	if _, ok := services["api"]; !ok {
		t.Error("expected api to be present")
	}
	if _, ok := services["redis"]; !ok {
		t.Error("expected redis to be present")
	}
	if _, ok := services["postgres"]; ok {
		t.Error("expected postgres to be removed")
	}
}

func TestServiceNames(t *testing.T) {
	composeYAML := `services:
  api:
    image: api:latest
  postgres:
    image: postgres:16
`
	names, err := ServiceNames([]byte(composeYAML))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 service names, got %d: %v", len(names), names)
	}
}

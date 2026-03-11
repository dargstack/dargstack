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
      - api.db_password.secret
    volumes:
      - api.data:/data
  postgres:
    image: postgres:16
    volumes:
      - postgres.data:/var/lib/postgresql/data
    secrets:
      - postgres.password.secret
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
      - monitoring.data:/data
secrets:
  api.db_password.secret:
    file: ./secrets/api.db_password.secret
  postgres.password.secret:
    file: ./secrets/postgres.password.secret
volumes:
  api.data:
  postgres.data:
  monitoring.data:
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
      - api.db_password.secret
    volumes:
      - api.data:/data
  postgres:
    image: postgres:16
    volumes:
      - postgres.data:/var/lib/postgresql/data
    secrets:
      - postgres.password.secret
  redis:
    image: redis:7
secrets:
  api.db_password.secret:
    file: ./secrets/api.db_password.secret
  postgres.password.secret:
    file: ./secrets/postgres.password.secret
volumes:
  api.data:
  postgres.data:
  redis.data:
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
	if _, ok := secrets["postgres.password.secret"]; ok {
		t.Error("expected postgres.password.secret to be removed")
	}

	volumes := doc["volumes"].(map[string]interface{})
	if _, ok := volumes["redis.data"]; ok {
		t.Error("expected redis.data volume to be removed")
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
      - api.secret
    deploy:
      labels:
        dargstack.profiles: default
  grafana:
    image: grafana:latest
    secrets:
      - grafana.webhook.secret
    deploy:
      labels:
        dargstack.profiles: monitoring
secrets:
  api.secret:
    file: ./secrets/api.secret
  grafana.webhook.secret:
    file: ./secrets/grafana.webhook.secret
x-dargstack:
  secrets:
    api.secret:
      type: random_string
    grafana.webhook.secret:
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
	if _, ok := secrets["api.secret"]; !ok {
		t.Error("expected api.secret template to be retained for active profile")
	}
	if _, ok := secrets["grafana.webhook.secret"]; ok {
		t.Error("expected grafana.webhook.secret template to be removed for inactive profile")
	}
}

func TestFilterByProfileLongFormSecretsKept(t *testing.T) {
	// Long-form secret reference: secrets: [{ source: name, target: /run/... }]
	composeYAML := `services:
  api:
    image: api:latest
    secrets:
      - source: api.db.secret
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
  api.db.secret:
    file: ./secrets/api.db.secret
  other.secret:
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
	if _, ok := secrets["api.db.secret"]; !ok {
		t.Error("expected api.db.secret to be kept (referenced via long form)")
	}
	if _, ok := secrets["other.secret"]; ok {
		t.Error("expected other.secret to be removed (unreferenced after profile filter)")
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

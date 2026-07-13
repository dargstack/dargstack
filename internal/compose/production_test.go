package compose

import (
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestRewriteProductionSecretsFileToExternal(t *testing.T) {
	input := `secrets:
  api-key:
    file: ./key.secret
  db-password:
    file: ./password.secret
`
	out, err := RewriteProductionSecrets([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}

	secrets, ok := doc["secrets"].(map[string]interface{})
	if !ok {
		t.Fatal("expected secrets map")
	}

	for _, name := range []string{"api-key", "db-password"} {
		def, ok := secrets[name].(map[string]interface{})
		if !ok {
			t.Fatalf("expected %s secret definition", name)
		}
		if _, hasFile := def["file"]; hasFile {
			t.Errorf("%s: file key should be removed", name)
		}
		ext, ok := def["external"].(bool)
		if !ok || !ext {
			t.Errorf("%s: expected external: true, got %v", name, def["external"])
		}
	}
}

func TestRewriteProductionSecretsAlreadyExternal(t *testing.T) {
	input := `secrets:
  api-key:
    external: true
  db-password:
    file: ./password.secret
`
	out, err := RewriteProductionSecrets([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}

	secrets := doc["secrets"].(map[string]interface{})
	apiKey := secrets["api-key"].(map[string]interface{})
	if _, hasFile := apiKey["file"]; hasFile {
		t.Error("already external secret should not get a file key")
	}
	ext, ok := apiKey["external"].(bool)
	if !ok || !ext {
		t.Error("already external secret should remain external: true")
	}

	dbPass := secrets["db-password"].(map[string]interface{})
	if _, hasFile := dbPass["file"]; hasFile {
		t.Error("db-password file key should be removed")
	}
}

func TestRewriteProductionSecretsNoSecrets(t *testing.T) {
	input := `services:
  web:
    image: nginx
`
	out, err := RewriteProductionSecrets([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}

	if _, hasSecrets := doc["secrets"]; hasSecrets {
		t.Error("should not add secrets section when none exists")
	}

	services, ok := doc["services"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services map")
	}
	if _, hasWeb := services["web"]; !hasWeb {
		t.Error("expected web service to be preserved")
	}
}

func TestRewriteProductionSecretsEmptySecrets(t *testing.T) {
	input := `secrets: {}
services:
  web:
    image: nginx
`
	out, err := RewriteProductionSecrets([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}

	if _, hasServices := doc["services"]; !hasServices {
		t.Error("services should be preserved")
	}
}

func TestRewriteProductionSecretsWithExtraKeys(t *testing.T) {
	input := `secrets:
  api-key:
    file: ./key.secret
    labels:
      environment: production
`
	out, err := RewriteProductionSecrets([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}

	secrets := doc["secrets"].(map[string]interface{})
	def := secrets["api-key"].(map[string]interface{})

	if _, hasFile := def["file"]; hasFile {
		t.Error("file key should be removed")
	}
	if ext, ok := def["external"].(bool); !ok || !ext {
		t.Error("external should be true")
	}
	if labels, ok := def["labels"]; !ok {
		t.Error("labels should be preserved")
	} else if labelsMap, ok := labels.(map[string]interface{}); !ok {
		t.Error("labels should be a map")
	} else if labelsMap["environment"] != "production" {
		t.Error("labels.environment should be preserved")
	}
}

package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dargstack/dargstack/v4/internal/secret"
)

// Issue represents a validation problem with a resource.
type Issue struct {
	Severity    string
	Resource    string
	Description string
}

func (i Issue) String() string {
	return fmt.Sprintf("[%s] %s: %s", strings.ToUpper(i.Severity), i.Resource, i.Description)
}

// Validate checks a compose file against the stack directory for missing resources.
func Validate(composeData []byte, stackDir string, production bool) ([]Issue, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return nil, fmt.Errorf("parse compose: %w", err)
	}

	var issues []Issue
	templates, _ := secret.ExtractTemplates(composeData)
	issues = append(issues, validateSecrets(doc, stackDir, production, templates)...)
	issues = append(issues, validateConfigs(doc, stackDir, production)...)
	issues = append(issues, validateServices(doc, stackDir, production)...)
	if !production {
		issues = append(issues, validateCertificates(stackDir)...)
	}
	return issues, nil
}

func validateSecrets(doc map[string]interface{}, stackDir string, production bool, templates map[string]secret.Template) []Issue {
	var issues []Issue

	secrets, ok := doc["secrets"].(map[string]interface{})
	if !ok {
		return nil
	}

	for name, def := range secrets {
		secretDef, ok := def.(map[string]interface{})
		if !ok {
			continue
		}

		if ext, ok := secretDef["external"].(bool); ok && ext {
			continue
		}

		// In production, secrets are managed externally by Docker Swarm.
		// File-path checks are only meaningful in development.
		if production {
			continue
		}

		if filePath, ok := secretDef["file"].(string); ok {
			// After merge, file paths are absolute (resolved from compose file directory).
			if _, err := os.Stat(filePath); err != nil {
				severity := "error"
				desc := fmt.Sprintf("file not found: %s", filePath)
				if !os.IsNotExist(err) {
					desc = fmt.Sprintf("cannot access file: %s (%v)", filePath, err)
				} else if tmpl, ok := templates[name]; ok && (tmpl.ThirdParty || tmpl.Type == secret.TypeThirdParty) {
					severity = "warning"
					desc = fmt.Sprintf("file not found (marked third_party): %s", filePath)
				}
				issues = append(issues, Issue{
					Severity:    severity,
					Resource:    fmt.Sprintf("secret:%s", name),
					Description: desc,
				})
			}
		}
	}

	return issues
}

func validateConfigs(doc map[string]interface{}, stackDir string, production bool) []Issue {
	var issues []Issue

	configs, ok := doc["configs"].(map[string]interface{})
	if !ok {
		return nil
	}

	for name, def := range configs {
		cfgDef, ok := def.(map[string]interface{})
		if !ok {
			continue
		}
		if ext, ok := cfgDef["external"].(bool); ok && ext {
			continue
		}
		// After merge, file paths are absolute (resolved from compose file directory).
		if filePath, ok := cfgDef["file"].(string); ok {
			if _, err := os.Stat(filePath); err != nil {
				desc := fmt.Sprintf("file not found: %s", filePath)
				if !os.IsNotExist(err) {
					desc = fmt.Sprintf("cannot access file: %s (%v)", filePath, err)
				}
				issues = append(issues, Issue{
					Severity:    "error",
					Resource:    fmt.Sprintf("config:%s", name),
					Description: desc,
				})
			}
		}
	}
	return issues
}

func validateServices(doc map[string]interface{}, stackDir string, production bool) []Issue {
	var issues []Issue

	services, ok := doc["services"].(map[string]interface{})
	if !ok {
		return nil
	}

	for name, def := range services {
		svcDef, ok := def.(map[string]interface{})
		if !ok {
			continue
		}

		// In production, warn when a service has no explicit deploy.update_config.order.
		// Docker defaults to "stop-first" which causes downtime for replicated services.
		// Any explicit value ("start-first" or "stop-first") silences the warning —
		// the important thing is that the author has consciously chosen a policy.
		// Skip services without an image — they are dev-only stubs that won't be deployed.
		if production && hasImage(svcDef) && !hasUpdateOrder(svcDef) {
			issues = append(issues, Issue{
				Severity:    "warning",
				Resource:    fmt.Sprintf("service:%s", name),
				Description: `deploy.update_config.order is not set — use "start-first" for zero-downtime or "stop-first" for stateful services`,
			})
		}

		// dargstack.development.build labels are dev-only and stripped before
		// production deployment, so skip this check in production mode.
		if production {
			continue
		}

		contextPath := extractDargstackBuildLabel(svcDef)
		if contextPath == "" {
			continue
		}

		if !filepath.IsAbs(contextPath) {
			contextPath = filepath.Join(stackDir, "src", "development", name, contextPath)
		}
		dockerfile := filepath.Join(contextPath, "Dockerfile")
		if _, err := os.Stat(dockerfile); err != nil {
			desc := fmt.Sprintf("Dockerfile not found: %s", dockerfile)
			if !os.IsNotExist(err) {
				desc = fmt.Sprintf("cannot access Dockerfile: %s (%v)", dockerfile, err)
			}
			issues = append(issues, Issue{
				Severity:    "error",
				Resource:    fmt.Sprintf("service:%s", name),
				Description: desc,
			})
		}
	}

	return issues
}

// extractDargstackBuildLabel reads the dargstack.development.build label from deploy.labels.
func extractDargstackBuildLabel(svc map[string]interface{}) string {
	deploy, ok := svc["deploy"].(map[string]interface{})
	if !ok {
		return ""
	}
	labels, ok := deploy["labels"]
	if !ok {
		return ""
	}
	switch v := labels.(type) {
	case map[string]interface{}:
		if ctx, ok := v["dargstack.development.build"].(string); ok {
			return ctx
		}
	case []interface{}:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			if strings.HasPrefix(s, "dargstack.development.build=") {
				return strings.TrimPrefix(s, "dargstack.development.build=")
			}
		}
	}
	return ""
}

// hasUpdateOrder reports whether the service has an explicit deploy.update_config.order value.
// Any non-empty value ("start-first" or "stop-first") is accepted — the check only
// ensures the author has consciously chosen a rolling-update policy.
func hasUpdateOrder(svc map[string]interface{}) bool {
	deploy, ok := svc["deploy"].(map[string]interface{})
	if !ok {
		return false
	}
	updateConfig, ok := deploy["update_config"].(map[string]interface{})
	if !ok {
		return false
	}
	order, ok := updateConfig["order"].(string)
	return ok && order != ""
}

func hasImage(svc map[string]interface{}) bool {
	img, ok := svc["image"].(string)
	return ok && img != ""
}

func validateCertificates(stackDir string) []Issue {
	var issues []Issue

	certDir := filepath.Join(stackDir, "artifacts", "certificates")
	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		issues = append(issues, Issue{
			Severity:    "warning",
			Resource:    "certificates",
			Description: fmt.Sprintf("TLS certificates directory not found: %s", certDir),
		})
		return issues
	}

	entries, err := os.ReadDir(certDir)
	if err != nil {
		return issues
	}

	hasCert := false
	hasKey := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pem") || strings.HasSuffix(e.Name(), ".crt") {
			hasCert = true
		}
		if strings.HasSuffix(e.Name(), "-key.pem") || strings.HasSuffix(e.Name(), ".key") {
			hasKey = true
		}
	}

	if !hasCert {
		issues = append(issues, Issue{
			Severity:    "warning",
			Resource:    "certificates",
			Description: "no TLS certificate found (run `dargstack certify` to generate)",
		})
	}
	if !hasKey {
		issues = append(issues, Issue{
			Severity:    "warning",
			Resource:    "certificates",
			Description: "no TLS private key found (run `dargstack certify` to generate)",
		})
	}

	return issues
}

// MissingSecrets returns the names of secrets that are missing files.
func MissingSecrets(issues []Issue) []string {
	var missing []string
	for _, iss := range issues {
		if iss.Severity == "error" && strings.HasPrefix(iss.Resource, "secret:") {
			name := strings.TrimPrefix(iss.Resource, "secret:")
			missing = append(missing, name)
		}
	}
	return missing
}

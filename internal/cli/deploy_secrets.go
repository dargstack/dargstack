package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dargstack/dargstack/v4/internal/logger"
	"github.com/dargstack/dargstack/v4/internal/prompt"
	"github.com/dargstack/dargstack/v4/internal/resource"
	"github.com/dargstack/dargstack/v4/internal/secret"
)

// secretSetupFlow sets up secrets for deployment. Returns warnings as []resource.Issue,
// an error on failure, and a bool indicating whether all secrets are set (no missing,
// no placeholders). When deployMode is true, tip messages are suppressed because the
// resource validator will report them in its structured output.
func secretSetupFlow(composeData []byte, prod, deployMode bool) ([]resource.Issue, error, bool) {
	templates, err := secret.ExtractTemplates(composeData)
	if err != nil || len(templates) == 0 {
		return nil, nil, true
	}

	if prod {
		// In production, third-party secrets that still hold the placeholder are an error.
		secretPaths := secret.ExtractSecretPaths(composeData)
		if len(secretPaths) > 0 {
			var placeholderThirdParty []string
			for name, tmpl := range templates {
				if !tmpl.ThirdParty && tmpl.Type != secret.TypeThirdParty {
					continue
				}
				if path, ok := secretPaths[name]; ok {
					data, err := os.ReadFile(path)
					if err != nil || secret.IsPlaceholderValue(strings.TrimSpace(string(data))) {
						placeholderThirdParty = append(placeholderThirdParty, name)
					}
				}
			}
			if len(placeholderThirdParty) > 0 {
				sort.Strings(placeholderThirdParty)
				return nil, hintErr(
					fmt.Errorf("production deployment blocked: third-party secrets still hold placeholder values: %s", strings.Join(placeholderThirdParty, ", ")),
					MsgReplaceSecretFiles,
				), false
			}
		}
		return nil, nil, true
	}

	// Extract file paths from compose for reading/writing secret values
	secretPaths := secret.ExtractSecretPaths(composeData)
	if len(secretPaths) == 0 {
		return nil, nil, true
	}

	// Read existing values
	values := secret.ReadSecretValues(secretPaths)
	preResolved := make(map[string]string, len(values))
	for k, v := range values {
		preResolved[k] = v
	}

	thirdParty := make(map[string]bool)
	for name, tmpl := range templates {
		if tmpl.ThirdParty || tmpl.Type == secret.TypeThirdParty {
			thirdParty[name] = true
		}
	}

	// Secrets whose file does not exist at all.
	missing := make([]string, 0, len(secretPaths))
	for name := range secretPaths {
		if secret.SecretFileExists(secretPaths, name) {
			continue
		}
		if !thirdParty[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)

	if len(missing) == 0 {
		var issues []resource.Issue
		if len(thirdParty) > 0 {
			var noFile []string
			var placeholder []string
			for name := range thirdParty {
				if !secret.SecretFileExists(secretPaths, name) {
					noFile = append(noFile, name)
				} else if secret.IsPlaceholderValue(values[name]) {
					placeholder = append(placeholder, name)
				}
			}
			if len(noFile) > 0 {
				sort.Strings(noFile)
				for _, name := range noFile {
					if tmpl, ok := templates[name]; ok && tmpl.Hint != "" {
						logger.L.Info(fmt.Sprintf("  %s: expected value — %s", name, tmpl.Hint))
					}
					values[name] = secret.ThirdPartyPlaceholder
				}
			}
			noFile = append(noFile, placeholder...)
			if len(noFile) > 0 {
				sort.Strings(noFile)
				for _, name := range noFile {
					issues = append(issues, resource.Issue{
						Severity:    "warning",
						Resource:    fmt.Sprintf("secret:%s", name),
						Description: "third-party secret not set",
					})
				}
				if !deployMode {
					logger.L.Info(MsgReplaceSecretFiles)
				}
			}
		}

		// Resolve templates (e.g., aws-credentials template that depends on third-party secrets).
		// Use ResolveAllowPlaceholders so templates referencing unset third-party secrets
		// resolve with placeholder values instead of failing.
		values, err = secret.ResolveAllowPlaceholders(templates, values)
		if err != nil {
			return nil, err, false
		}

		if err := secret.WriteSecrets(secretPaths, values); err != nil {
			return nil, err, false
		}

		allSet := checkAllSecretsSet(secretPaths, values, thirdParty)
		return issues, nil, allSet
	}

	if noInteraction {
		// Pre-populate third-party placeholders so templates that depend on
		// them can resolve without error. The user will be warned below.
		for name := range thirdParty {
			if values[name] == "" {
				values[name] = secret.ThirdPartyPlaceholder
			}
		}

		// Auto-generate what we can without interaction.
		values, err = secret.ResolveAllowPlaceholders(templates, values)
		if err != nil {
			return nil, err, false
		}

		// Count auto-generated secrets.
		autoResolvedCount := 0
		for name, tmpl := range templates {
			if preResolved[name] != "" {
				continue
			}
			if values[name] == "" {
				continue
			}
			if secret.IsAutoGeneratable(&tmpl) {
				autoResolvedCount++
			}
		}
		if autoResolvedCount > 0 {
			logger.L.Info(fmt.Sprintf("Auto-generated %d secret(s) from x-dargstack.secrets", autoResolvedCount))
			logger.L.Info("Review generated values with `dargstack secret show`.")
		}

		// Determine what's still missing (no value and not third-party).
		stillMissing := make([]string, 0, len(secretPaths))
		for name := range secretPaths {
			if values[name] == "" && !thirdParty[name] {
				stillMissing = append(stillMissing, name)
			}
		}
		sort.Strings(stillMissing)

		// Collect warnings for third-party secrets that still have placeholders.
		unsetThirdParty := make([]string, 0, len(thirdParty))
		for name := range thirdParty {
			if values[name] == "" || secret.IsPlaceholderValue(values[name]) {
				unsetThirdParty = append(unsetThirdParty, name)
			}
		}
		sort.Strings(unsetThirdParty)

		var issues []resource.Issue
		if len(unsetThirdParty) > 0 {
			for _, name := range unsetThirdParty {
				issues = append(issues, resource.Issue{
					Severity:    "warning",
					Resource:    fmt.Sprintf("secret:%s", name),
					Description: "third-party secret not set",
				})
			}
			if !deployMode {
				logger.L.Info(MsgReplaceSecretFiles)
			}
		}

		if len(stillMissing) > 0 {
			for _, name := range stillMissing {
				issues = append(issues, resource.Issue{
					Severity:    "warning",
					Resource:    fmt.Sprintf("secret:%s", name),
					Description: "secret not set — run interactively to set it",
				})
			}
			// Only show tip if any missing secret lacks an x-dargstack.secrets definition.
			hasNoTemplate := false
			for _, name := range stillMissing {
				if _, ok := templates[name]; !ok {
					hasNoTemplate = true
					break
				}
			}
			if hasNoTemplate && !deployMode {
				logger.L.Info(TipAddSecretMetadata)
			}
		}

		if err := secret.WriteSecrets(secretPaths, values); err != nil {
			return nil, err, false
		}

		return issues, nil, len(stillMissing) == 0 && len(unsetThirdParty) == 0
	}

	manualInput := make(map[string]bool)
	skipAllAuto := false

	for _, name := range missing {
		if values[name] != "" {
			continue
		}

		tmpl, hasTemplate := templates[name]
		autoResolvable := hasTemplate && secret.IsAutoGeneratable(&tmpl)

		if autoResolvable {
			if skipAllAuto {
				continue
			}

			choice, choiceErr := prompt.Select(
				fmt.Sprintf("Secret %s", name),
				[]string{
					"Enter value",
					ChoiceAutoGenThis,
					ChoiceAutoGenAll,
				},
			)
			if choiceErr != nil {
				return nil, choiceErr, false
			}

			if choice == ChoiceAutoGenAll {
				skipAllAuto = true
				continue
			}
			if choice == ChoiceAutoGenThis {
				continue
			}
		}

		promptText := fmt.Sprintf("Value for secret %s:", name)
		val, inputErr := prompt.Password(promptText)
		if inputErr != nil {
			return nil, inputErr, false
		}
		if val != "" {
			values[name] = val
			manualInput[name] = true
		}
	}

	// Pre-populate third-party placeholders so templates that depend on them
	// resolve without error.
	for name := range thirdParty {
		if values[name] == "" {
			values[name] = secret.ThirdPartyPlaceholder
		}
	}

	// Auto-generate any values that can be derived from x-dargstack.secrets.
	// Use ResolveAllowPlaceholders so templates referencing unset third-party
	// secrets resolve with placeholder values instead of failing.
	values, err = secret.ResolveAllowPlaceholders(templates, values)
	if err != nil {
		return nil, err, false
	}

	autoResolvedCount := 0
	for name, tmpl := range templates {
		if preResolved[name] != "" || manualInput[name] {
			continue
		}
		if values[name] == "" {
			continue
		}
		if secret.IsAutoGeneratable(&tmpl) {
			autoResolvedCount++
		}
	}
	if autoResolvedCount > 0 {
		logger.L.Info(fmt.Sprintf("Auto-generated %d secret(s) from x-dargstack.secrets", autoResolvedCount))
		logger.L.Info("Review generated values with `dargstack secret show`.")
	}

	stillMissing := make([]string, 0, len(secretPaths))
	for name := range secretPaths {
		if values[name] == "" && !thirdParty[name] {
			stillMissing = append(stillMissing, name)
		}
	}
	sort.Strings(stillMissing)

	// Collect warnings for third-party secrets that still have placeholders.
	unsetThirdParty := make([]string, 0, len(thirdParty))
	for name := range thirdParty {
		if values[name] == "" || secret.IsPlaceholderValue(values[name]) {
			unsetThirdParty = append(unsetThirdParty, name)
		}
	}
	sort.Strings(unsetThirdParty)

	var issues []resource.Issue
	if len(unsetThirdParty) > 0 {
		for _, name := range unsetThirdParty {
			issues = append(issues, resource.Issue{
				Severity:    "warning",
				Resource:    fmt.Sprintf("secret:%s", name),
				Description: "third-party secret not set",
			})
		}
		if !deployMode {
			logger.L.Info(MsgReplaceSecretFiles)
		}
	}

	if len(stillMissing) > 0 {
		for _, name := range stillMissing {
			issues = append(issues, resource.Issue{
				Severity:    "warning",
				Resource:    fmt.Sprintf("secret:%s", name),
				Description: "secret not set",
			})
		}
		// Only show tip if any missing secret lacks an x-dargstack.secrets definition.
		hasNoTemplate := false
		for _, name := range stillMissing {
			if _, ok := templates[name]; !ok {
				hasNoTemplate = true
				break
			}
		}
		if hasNoTemplate && !deployMode {
			logger.L.Info(TipAddSecretMetadata)
		}
	}

	if err := secret.WriteSecrets(secretPaths, values); err != nil {
		return nil, err, false
	}

	allSet := checkAllSecretsSet(secretPaths, values, thirdParty)
	return issues, nil, allSet
}

// checkAllSecretsSet returns true if all secrets have real values (no missing, no placeholders).
func checkAllSecretsSet(secretPaths, values map[string]string, thirdParty map[string]bool) bool {
	for name := range secretPaths {
		v := values[name]
		if v == "" || secret.IsPlaceholderValue(v) || strings.Contains(v, secret.ThirdPartyPlaceholder) {
			return false
		}
	}
	return true
}

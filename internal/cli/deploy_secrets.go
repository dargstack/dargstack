package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dargstack/dargstack/v4/internal/prompt"
	"github.com/dargstack/dargstack/v4/internal/secret"
)

func secretSetupFlow(composeData []byte, prod bool) error {
	templates, err := secret.ExtractTemplates(composeData)
	if err != nil || len(templates) == 0 {
		return nil
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
				return hintErr(
					fmt.Errorf("production deployment blocked: third-party secrets still hold placeholder values: %s", strings.Join(placeholderThirdParty, ", ")),
					"Replace those secret files with real values before deploying to production.",
				)
			}
		}
		return nil
	}

	// Extract file paths from compose for reading/writing secret values
	secretPaths := secret.ExtractSecretPaths(composeData)
	if len(secretPaths) == 0 {
		return nil
	}

	// Read existing values
	values := secret.ReadSecretValues(secretPaths)
	preResolved := make(map[string]string, len(values))
	for k, v := range values {
		preResolved[k] = v
	}

	missing := make([]string, 0, len(secretPaths))
	thirdParty := make(map[string]bool)
	for name, tmpl := range templates {
		if tmpl.ThirdParty || tmpl.Type == secret.TypeThirdParty {
			thirdParty[name] = true
		}
	}
	for name := range secretPaths {
		// Only prompt for secrets whose file does not exist at all.
		// Files that exist (even with placeholder content) are not reprompted —
		// third-party placeholders warn on deploy; other placeholders warn below.
		if secret.SecretFileExists(secretPaths, name) {
			continue
		}
		if !thirdParty[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)

	if len(missing) == 0 {
		if len(thirdParty) > 0 {
			var noFile []string
			// Write UNSET placeholder for any third-party secret with no file at all.
			for name := range thirdParty {
				if !secret.SecretFileExists(secretPaths, name) {
					noFile = append(noFile, name)
				}
			}
			if len(noFile) > 0 {
				sort.Strings(noFile)
				printInfo("Skipping local generation for third-party secrets")
				for _, name := range noFile {
					if tmpl, ok := templates[name]; ok && tmpl.Hint != "" {
						printInfo(fmt.Sprintf("  %s: expected value — %s", name, tmpl.Hint))
					}
					values[name] = secret.ThirdPartyPlaceholder
				}
				printWarning(fmt.Sprintf("Third-party secrets still unset: %s", strings.Join(noFile, ", ")))
				printInfo("Replace those secret files with real values before deploying to production.")
				_ = secret.WriteSecrets(secretPaths, values)
			}
		}
		return nil
	}

	if noInteraction {
		printWarning(fmt.Sprintf("Unset secrets: %s — run interactively to set them", strings.Join(missing, ", ")))
		printInfo("Tip: Add x-dargstack.secrets entries with typed secret metadata to auto-generate missing secrets.")
		return nil
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
					"Auto-generate this and remaining auto-generatable secrets (skip all)",
					"Auto-generate this secret (skip)",
					"Enter value",
				},
			)
			if choiceErr != nil {
				return choiceErr
			}

			if choice == "Auto-generate this and remaining auto-generatable secrets (skip all)" {
				skipAllAuto = true
				continue
			}
			if choice == "Auto-generate this secret (skip)" {
				continue
			}
		}

		promptText := fmt.Sprintf("Value for secret %s:", name)
		val, inputErr := prompt.Password(promptText)
		if inputErr != nil {
			return inputErr
		}
		if val != "" {
			values[name] = val
			manualInput[name] = true
		}
	}

	// Auto-generate any values that can be derived from x-dargstack.secrets.
	values, err = secret.Resolve(templates, values)
	if err != nil {
		return err
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
		printInfo(fmt.Sprintf("Auto-generated %d secret(s) from x-dargstack.secrets", autoResolvedCount))
		printInfo("Review generated values with `dargstack deploy --list-secrets`.")
	}

	// Write UNSET THIRD PARTY SECRET placeholder for third-party secrets that still have no file.
	for name := range thirdParty {
		if !secret.SecretFileExists(secretPaths, name) {
			values[name] = secret.ThirdPartyPlaceholder
		}
	}

	stillMissing := make([]string, 0, len(secretPaths))
	for name := range secretPaths {
		if values[name] == "" && !thirdParty[name] {
			stillMissing = append(stillMissing, name)
		}
	}
	sort.Strings(stillMissing)
	if len(stillMissing) > 0 {
		printWarning(fmt.Sprintf("Unset secrets remain: %s", strings.Join(stillMissing, ", ")))
		printInfo("Tip: Add x-dargstack.secrets entries with typed secret metadata to auto-generate missing secrets.")
	}

	return secret.WriteSecrets(secretPaths, values)
}

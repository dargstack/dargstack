package cli

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/v4/internal/prompt"
	"github.com/dargstack/dargstack/v4/internal/secret"
)

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage stack secrets",
	Long: `Manage stack secrets.

List, inspect, generate, and check the status of secrets defined in your stack.

Use 'dargstack secret generate' to create secrets from x-dargstack.secrets templates.
Use 'dargstack secret show' to view secret values (with clipboard support if available).
Use 'dargstack secret show --type key' to derive public keys from private_key type secrets.
Use 'dargstack secret status' to check which secrets are set, missing, or hold placeholders.`,
}

var secretListCmd = &cobra.Command{
	Use:   "list",
	Short: "List secret names and file paths",
	Long: `List all secret names and their file paths.

Without flags, lists all secret names and their file paths in the current stack.`,
	RunE: runSecretList,
}

var secretShowType string

var secretShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show secret values",
	Long: `Show secret values.

Displays the current values of all secrets. If a clipboard tool is available
(wl-copy, xclip, xsel, pbcopy, clip), offers an interactive picker to copy
individual keys and values.

If a secret name is provided, only that secret is shown.

Use --type key to derive and display public keys for private_key type secrets
instead of showing stored values.`,
	RunE: runSecretShow,
}

var secretGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate secrets from x-dargstack.secrets templates",
	Long: `Generate secrets from x-dargstack.secrets templates.

Reads secret templates from the compose file and generates values for any
missing secrets. Auto-generatable types (random_string, wordlist_word,
private_key, insecure_default, template) are created automatically.
Third-party secrets require manual values.

In production mode (--production), validates that third-party secrets do not
hold placeholder values and blocks if they do.

In non-interactive mode (--no-interaction), auto-generates what it can and
warns about secrets that still need values.`,
	RunE: runSecretGenerate,
}

var secretStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show secret status",
	Long: `Show which secrets are set, missing, or hold placeholder values.

Displays the status of each secret:
  set        - has a real value on disk
  placeholder - holds a third-party placeholder value
  missing    - no file exists on disk`,
	RunE: runSecretStatus,
}

func init() {
	secretCmd.AddCommand(secretListCmd)
	secretCmd.AddCommand(secretShowCmd)
	secretCmd.AddCommand(secretGenerateCmd)
	secretCmd.AddCommand(secretStatusCmd)

	secretListCmd.Flags().BoolVarP(&production, "production", "p", false, "use production compose")
	secretShowCmd.Flags().BoolVarP(&production, "production", "p", false, "use production compose")
	secretShowCmd.Flags().StringVar(&secretShowType, "type", "value", "output type: value (secret values) or key (derived public keys)")
	secretGenerateCmd.Flags().BoolVarP(&production, "production", "p", false, "use production compose")
	secretGenerateCmd.Flags().StringSliceVar(&profiles, "profiles", nil, FlagDescProfiles)
	secretStatusCmd.Flags().BoolVarP(&production, "production", "p", false, "use production compose")
}

func runSecretList(_ *cobra.Command, _ []string) error {
	composeData, err := buildComposeData(production)
	if err != nil {
		return wrapWithBugHint(err)
	}

	paths := secret.ExtractSecretPaths(composeData)
	if len(paths) == 0 {
		printInfo("No secrets found")
		return nil
	}

	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	sort.Strings(names)

	jsonOutput := noInteraction || strings.EqualFold(outputFormat, "json")
	if jsonOutput {
		entries := make([]map[string]string, 0, len(names))
		for _, name := range names {
			entries = append(entries, map[string]string{
				"name": name,
				"file": paths[name],
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	nameWidth := len("NAME")
	for _, name := range names {
		if len(name) > nameWidth {
			nameWidth = len(name)
		}
	}
	fmt.Printf("%-*s  %s\n", nameWidth, "NAME", "FILE")
	fmt.Printf("%-*s  %s\n", nameWidth, strings.Repeat("-", nameWidth), strings.Repeat("-", 4))
	for _, name := range names {
		fmt.Printf("%-*s  %s\n", nameWidth, name, paths[name])
	}
	return nil
}

func runSecretShow(_ *cobra.Command, args []string) error {
	var targetName string
	if len(args) > 0 {
		targetName = args[0]
	}
	if secretShowType == "key" {
		return runSecretShowKeys(targetName)
	}
	return runSecretShowValues(targetName)
}

func runSecretShowValues(targetName string) error {
	composeData, err := buildComposeData(production)
	if err != nil {
		return wrapWithBugHint(err)
	}

	paths := secret.ExtractSecretPaths(composeData)
	if len(paths) == 0 {
		printInfo("No secrets found")
		return nil
	}

	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	sort.Strings(names)

	if targetName != "" {
		found := false
		for _, name := range names {
			if name == targetName {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("secret %q not found", targetName)
		}
		names = []string{targetName}
	}

	values := secret.ReadSecretValues(paths)

	jsonOutput := noInteraction || strings.EqualFold(outputFormat, "json")
	switch {
	case jsonOutput:
		entries := make([]map[string]string, 0, len(names))
		for _, name := range names {
			entries = append(entries, map[string]string{
				"name":  name,
				"file":  paths[name],
				"value": values[name],
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)

	case hasClipboardSupport():
		for i, name := range names {
			for {
				title := fmt.Sprintf("Secret %d/%d: %s", i+1, len(names), name)
				choice, choiceErr := prompt.Select(title, []string{
					ChoiceCopyKey,
					ChoiceCopyValue,
					"Next",
					"Done",
				})
				if choiceErr != nil {
					return choiceErr
				}
				switch choice {
				case ChoiceCopyKey:
					if copyErr := copyToClipboard(name); copyErr != nil {
						printWarning(fmt.Sprintf(MsgClipboardCopyFailed, copyErr))
					} else {
						printSuccess(fmt.Sprintf("Copied key %q", name))
					}
				case ChoiceCopyValue:
					if copyErr := copyToClipboard(values[name]); copyErr != nil {
						printWarning(fmt.Sprintf(MsgClipboardCopyFailed, copyErr))
					} else {
						printSuccess(fmt.Sprintf("Copied value for %q", name))
					}
				case "Done":
					return nil
				default: // Next
					goto nextSecret
				}
			}
		nextSecret:
		}

	default:
		printWarning("No clipboard tool found. Falling back to table output.")
		nameWidth := len("NAME")
		for _, name := range names {
			if len(name) > nameWidth {
				nameWidth = len(name)
			}
		}
		fmt.Printf("%-*s  %s\n", nameWidth, "NAME", "VALUE")
		fmt.Printf("%-*s  %s\n", nameWidth, strings.Repeat("-", nameWidth), strings.Repeat("-", 5))
		for _, name := range names {
			fmt.Printf("%-*s  %s\n", nameWidth, name, values[name])
		}
	}
	return nil
}

func runSecretShowKeys(targetName string) error {
	composeData, err := buildComposeData(production)
	if err != nil {
		return wrapWithBugHint(err)
	}

	templates, err := secret.ExtractTemplates(composeData)
	if err != nil {
		return fmt.Errorf("extract secret templates: %w", err)
	}

	paths := secret.ExtractSecretPaths(composeData)
	if len(paths) == 0 {
		printInfo("No secrets found")
		return nil
	}

	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	sort.Strings(names)

	if targetName != "" {
		found := false
		for _, name := range names {
			if name == targetName {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("secret %q not found", targetName)
		}
		names = []string{targetName}
	}

	type pubEntry struct {
		KeyType   string `json:"key_type"`
		Name      string `json:"name"`
		PublicKey string `json:"public_key"`
	}

	var entries []pubEntry
	for _, name := range names {
		tmpl, ok := templates[name]
		if !ok || tmpl.Type != secret.TypePrivateKey {
			continue
		}

		filePath := paths[name]

		data, readErr := os.ReadFile(filePath)
		if readErr != nil {
			printWarning(fmt.Sprintf("Cannot read %s: %v", name, readErr))
			continue
		}

		pub, keyType, deriveErr := derivePublicKeyPEM(data)
		if deriveErr != nil {
			printWarning(fmt.Sprintf("Cannot derive public key for %s: %v", name, deriveErr))
			continue
		}

		entries = append(entries, pubEntry{Name: name, KeyType: keyType, PublicKey: pub})
	}

	if len(entries) == 0 {
		printInfo("No private_key type secrets with values found")
		return nil
	}

	jsonOutput := noInteraction || strings.EqualFold(outputFormat, "json")
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	for _, e := range entries {
		fmt.Printf("=== %s (%s) ===\n%s\n", e.Name, e.KeyType, e.PublicKey)
	}
	return nil
}

func runSecretGenerate(_ *cobra.Command, _ []string) error {
	composeData, err := buildComposeData(production)
	if err != nil {
		return wrapWithBugHint(err)
	}

	composeData, err = applyProfileFilter(composeData)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrFilterComposeByProfile, err)
	}

	if err := secretSetupFlow(composeData, production); err != nil {
		return err
	}

	printSuccess("Secret generation complete. Run `dargstack deploy` to deploy.")
	return nil
}

func runSecretStatus(_ *cobra.Command, _ []string) error {
	composeData, err := buildComposeData(production)
	if err != nil {
		return wrapWithBugHint(err)
	}

	paths := secret.ExtractSecretPaths(composeData)
	if len(paths) == 0 {
		printInfo("No secrets found")
		return nil
	}

	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	sort.Strings(names)

	values := secret.ReadSecretValues(paths)

	type statusEntry struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		File   string `json:"file"`
	}

	var entries []statusEntry
	for _, name := range names {
		status := "missing"
		if secret.SecretFileExists(paths, name) {
			status = "set"
			if secret.IsPlaceholderValue(strings.TrimSpace(values[name])) {
				status = "placeholder"
			}
		}
		entries = append(entries, statusEntry{Name: name, Status: status, File: paths[name]})
	}

	jsonOutput := noInteraction || strings.EqualFold(outputFormat, "json")
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	nameWidth := len("NAME")
	for _, e := range entries {
		if len(e.Name) > nameWidth {
			nameWidth = len(e.Name)
		}
	}
	fmt.Printf("%-*s  %-11s  %s\n", nameWidth, "NAME", "STATUS", "FILE")
	fmt.Printf("%-*s  %-11s  %s\n", nameWidth, strings.Repeat("-", nameWidth), "-------", "----")
	for _, e := range entries {
		fmt.Printf("%-*s  %-11s  %s\n", nameWidth, e.Name, e.Status, e.File)
	}

	// Summary counts
	var missing, placeholder, set int
	for _, e := range entries {
		switch e.Status {
		case "missing":
			missing++
		case "placeholder":
			placeholder++
		case "set":
			set++
		}
	}

	if missing > 0 || placeholder > 0 {
		fmt.Fprintln(os.Stderr, "")
		if missing > 0 {
			printWarning(fmt.Sprintf("%d secret(s) missing", missing))
		}
		if placeholder > 0 {
			printWarning(fmt.Sprintf("%d secret(s) hold placeholder values", placeholder))
		}
		printInfo("Run `dargstack secret generate` to create missing secrets.")
		return fmt.Errorf("not all secrets are set")
	}

	printSuccess(fmt.Sprintf("All %d secret(s) are set", set))
	return nil
}

// derivePublicKeyPEM reads a PEM-encoded private key and returns the public key
// as a PEM string together with a human-readable algorithm label.
func derivePublicKeyPEM(data []byte) (pubPEM, keyType string, err error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return "", "", fmt.Errorf("no PEM block found")
	}

	var privKey interface{}
	switch block.Type {
	case "EC PRIVATE KEY":
		privKey, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		privKey, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	}
	if err != nil {
		return "", "", fmt.Errorf("parse private key: %w", err)
	}

	var pub interface{}
	switch k := privKey.(type) {
	case ed25519.PrivateKey:
		pub = k.Public()
		keyType = "ed25519"
	case *rsa.PrivateKey:
		pub = &k.PublicKey
		keyType = fmt.Sprintf("rsa-%d", k.N.BitLen())
	case *ecdsa.PrivateKey:
		pub = &k.PublicKey
		keyType = fmt.Sprintf("ecdsa-p%d", k.Curve.Params().BitSize)
	default:
		return "", "", fmt.Errorf("unsupported key type %T", privKey)
	}

	der, marshalErr := x509.MarshalPKIXPublicKey(pub)
	if marshalErr != nil {
		return "", "", fmt.Errorf("marshal public key: %w", marshalErr)
	}

	pubPEM = strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})))
	return pubPEM, keyType, nil
}
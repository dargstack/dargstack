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

	"github.com/dargstack/dargstack/v4/internal/config"
	"github.com/dargstack/dargstack/v4/internal/prompt"
	"github.com/dargstack/dargstack/v4/internal/secret"
)

var (
	secretShow      bool
	secretPublicKey bool
)

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Inspect stack secrets",
	Long: `Inspect stack secrets.

Without flags, lists all secret names and their file paths.
Use --show to include values (with clipboard support if available).
Use --public-key to derive and display the public key for private_key type secrets.`,
	RunE: runSecret,
}

func init() {
	secretCmd.Flags().BoolVarP(&secretShow, "show", "s", false, "show secret values")
	secretCmd.Flags().BoolVarP(&secretPublicKey, "public-key", "k", false, "show public keys for private_key type secrets")
	secretCmd.Flags().BoolVarP(&production, "production", "p", false, "use production compose")
}

func runSecret(_ *cobra.Command, _ []string) error {
	var composeData []byte
	var err error
	if production {
		composeData, err = buildProductionCompose()
	} else {
		composeData, err = buildDevelopmentCompose()
	}
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

	if secretPublicKey {
		return runSecretPublicKeys(composeData, names, paths)
	}

	if secretShow {
		return runSecretShow(names, paths)
	}

	// Default: names and paths only.
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

// runSecretShow replicates the former deploy --list-secrets behaviour: values
// with clipboard support when available, falling back to a table.
func runSecretShow(names []string, paths map[string]string) error {
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
					"Copy key to clipboard",
					"Copy value to clipboard",
					"Next",
					"Done",
				})
				if choiceErr != nil {
					return choiceErr
				}
				switch choice {
				case "Copy key to clipboard":
					if copyErr := copyToClipboard(name); copyErr != nil {
						printWarning(fmt.Sprintf("Clipboard copy failed: %v", copyErr))
					} else {
						printSuccess(fmt.Sprintf("Copied key %q", name))
					}
				case "Copy value to clipboard":
					if copyErr := copyToClipboard(values[name]); copyErr != nil {
						printWarning(fmt.Sprintf("Clipboard copy failed: %v", copyErr))
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

// runSecretPublicKeys extracts and prints the public key for every private_key
// type secret that has a value on disk.
func runSecretPublicKeys(composeData []byte, names []string, paths map[string]string) error {
	templates, err := secret.ExtractTemplates(composeData)
	if err != nil {
		return fmt.Errorf("extract secret templates: %w", err)
	}

	type pubEntry struct {
		Name      string `json:"name"`
		KeyType   string `json:"key_type"`
		PublicKey string `json:"public_key"`
	}

	var entries []pubEntry
	for _, name := range names {
		tmpl, ok := templates[name]
		if !ok || tmpl.Type != secret.TypePrivateKey {
			continue
		}

		filePath := paths[name]
		if filePath == "" {
			filePath = config.SecretsDir(stackDir) + "/" + name
		}

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

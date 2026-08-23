package secret

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
)

// DerivePublicKeyPEM reads a PEM-encoded private key and returns the public key as a PEM string together with a human-readable algorithm label.
func DerivePublicKeyPEM(data []byte) (pubPEM, keyType string, err error) {
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

// ResolveConfigs derives values for x-dargstack.configs templates.
// Currently the only supported type is public_key, which derives a public key PEM from the resolved value of the private_key secret named by Source.
// Entries whose source secret isn't resolved yet are left out of the result rather than erroring, so callers can retry once the secret is generated.
func ResolveConfigs(configTemplates map[string]Template, secretValues map[string]string) (map[string]string, error) {
	values := make(map[string]string, len(configTemplates))
	for name := range configTemplates {
		tmpl := configTemplates[name]
		normalizeTemplate(&tmpl)
		if tmpl.Type != TypePublicKey {
			continue
		}

		source := strings.TrimSpace(tmpl.Source)
		if source == "" {
			return nil, fmt.Errorf("config %s: public_key type requires a source", name)
		}

		privValue, ok := secretValues[source]
		if !ok || privValue == "" || IsPlaceholderValue(privValue) {
			continue
		}

		pub, _, err := DerivePublicKeyPEM([]byte(privValue))
		if err != nil {
			return nil, fmt.Errorf("derive public key for config %s: %w", name, err)
		}
		values[name] = pub
	}
	return values, nil
}

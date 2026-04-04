package secret

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
)

func generateRandom(length int, includeSpecial bool) (string, error) {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if includeSpecial {
		charset += "!@#$%^&*()-_=+[]{}:,.?"
	}

	out := make([]byte, length)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		out[i] = charset[n.Int64()]
	}
	return string(out), nil
}

func generateWord() (string, error) {
	adjectives := []string{"amber", "brisk", "calm", "daring", "ember", "frost", "gentle", "hazel", "ivory", "jolly"}
	nouns := []string{"falcon", "harbor", "island", "jungle", "keystone", "lantern", "meadow", "nebula", "orchid", "pioneer"}

	a, err := rand.Int(rand.Reader, big.NewInt(int64(len(adjectives))))
	if err != nil {
		return "", err
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(nouns))))
	if err != nil {
		return "", err
	}
	return adjectives[a.Int64()] + "-" + nouns[n.Int64()], nil
}

func generatePrivateKey(keyType string, keySize int) (string, error) {
	switch keyType {
	case "rsa":
		size := keySize
		if size <= 0 {
			size = 2048
		}
		key, err := rsa.GenerateKey(rand.Reader, size)
		if err != nil {
			return "", fmt.Errorf("generate RSA key: %w", err)
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return "", fmt.Errorf("marshal RSA key: %w", err)
		}
		p := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		return strings.TrimSpace(string(p)), nil
	case "ecdsa":
		var curve elliptic.Curve
		switch keySize {
		case 384:
			curve = elliptic.P384()
		case 521:
			curve = elliptic.P521()
		default:
			curve = elliptic.P256()
		}
		key, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			return "", fmt.Errorf("generate ECDSA key: %w", err)
		}
		der, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return "", fmt.Errorf("marshal ECDSA key: %w", err)
		}
		p := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
		return strings.TrimSpace(string(p)), nil
	default: // "ed25519" or empty
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return "", fmt.Errorf("generate Ed25519 key: %w", err)
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return "", fmt.Errorf("marshal Ed25519 key: %w", err)
		}
		p := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		return strings.TrimSpace(string(p)), nil
	}
}

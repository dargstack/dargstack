package cli

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func generateTestRSAKey(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal RSA key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func generateTestECKey(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal EC key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
}

func generateTestEd25519Key(t *testing.T) []byte {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal ed25519 key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func TestDerivePublicKeyPEM_RSA(t *testing.T) {
	data := generateTestRSAKey(t)
	pubPEM, keyType, err := derivePublicKeyPEM(data)
	if err != nil {
		t.Fatalf("derivePublicKeyPEM: %v", err)
	}
	if keyType != "rsa-2048" {
		t.Errorf("keyType = %q, want %q", keyType, "rsa-2048")
	}
	if !strings.Contains(pubPEM, "BEGIN PUBLIC KEY") {
		t.Error("PEM missing BEGIN PUBLIC KEY header")
	}
	if !strings.Contains(pubPEM, "END PUBLIC KEY") {
		t.Error("PEM missing END PUBLIC KEY header")
	}
}

func TestDerivePublicKeyPEM_EC(t *testing.T) {
	data := generateTestECKey(t)
	pubPEM, keyType, err := derivePublicKeyPEM(data)
	if err != nil {
		t.Fatalf("derivePublicKeyPEM: %v", err)
	}
	if keyType != "ecdsa-p256" {
		t.Errorf("keyType = %q, want %q", keyType, "ecdsa-p256")
	}
	if !strings.Contains(pubPEM, "BEGIN PUBLIC KEY") {
		t.Error("PEM missing BEGIN PUBLIC KEY header")
	}
	if !strings.Contains(pubPEM, "END PUBLIC KEY") {
		t.Error("PEM missing END PUBLIC KEY header")
	}
}

func TestDerivePublicKeyPEM_Ed25519(t *testing.T) {
	data := generateTestEd25519Key(t)
	pubPEM, keyType, err := derivePublicKeyPEM(data)
	if err != nil {
		t.Fatalf("derivePublicKeyPEM: %v", err)
	}
	if keyType != "ed25519" {
		t.Errorf("keyType = %q, want %q", keyType, "ed25519")
	}
	if !strings.Contains(pubPEM, "BEGIN PUBLIC KEY") {
		t.Error("PEM missing BEGIN PUBLIC KEY header")
	}
	if !strings.Contains(pubPEM, "END PUBLIC KEY") {
		t.Error("PEM missing END PUBLIC KEY header")
	}
}

func TestDerivePublicKeyPEM_Errors(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		errWant string
	}{
		{
			name:    "not PEM",
			data:    []byte("not a PEM block"),
			errWant: "no PEM block found",
		},
		{
			name:    "empty data",
			data:    []byte{},
			errWant: "no PEM block found",
		},
		{
			name:    "invalid key bytes",
			data:    pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("invalid")}),
			errWant: "parse private key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := derivePublicKeyPEM(tt.data)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errWant) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.errWant)
			}
		})
	}
}

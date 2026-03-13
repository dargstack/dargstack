package secret

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
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

func generatePrivateKey() (string, error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", err
	}
	p := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return strings.TrimSpace(string(p)), nil
}

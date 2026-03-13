package tls

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateSelfSigned(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	if err := generateSelfSigned(certFile, keyFile, []string{"localhost", "127.0.0.1"}); err != nil {
		t.Fatal(err)
	}

	// Verify cert file
	certData, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certData)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("expected PEM CERTIFICATE block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	if cert.Subject.Organization[0] != "dargstack development" {
		t.Errorf("unexpected org: %s", cert.Subject.Organization[0])
	}
	if cert.NotAfter.Before(time.Now().Add(364 * 24 * time.Hour)) {
		t.Error("certificate validity too short")
	}
	if cert.DNSNames[0] != "localhost" {
		t.Errorf("expected SAN localhost, got %v", cert.DNSNames)
	}
	if cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Error("expected digital signature key usage")
	}

	// Verify key file
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil || keyBlock.Type != "EC PRIVATE KEY" {
		t.Fatal("expected PEM EC PRIVATE KEY block")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if key.Curve.Params().BitSize != 256 {
		t.Errorf("expected P-256, got %d-bit curve", key.Curve.Params().BitSize)
	}

	// Verify public key matches
	pubFromCert, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		t.Fatal("expected ECDSA public key in cert")
	}
	if !pubFromCert.Equal(&key.PublicKey) {
		t.Error("cert public key does not match private key")
	}

	// Verify key file permissions
	info, _ := os.Stat(keyFile)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected key file permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestEnsureCertificatesSkipsValidExisting(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "localhost.pem")
	keyFile := filepath.Join(dir, "localhost-key.pem")

	// Generate a valid cert first
	domains := []string{"localhost", "127.0.0.1"}
	if err := generateSelfSigned(certFile, keyFile, domains); err != nil {
		t.Fatal(err)
	}

	// Record modification time
	certInfo, _ := os.Stat(certFile)
	origModTime := certInfo.ModTime()

	// Call EnsureCertificates with same domains — should not regenerate
	if err := EnsureCertificates(dir, domains); err != nil {
		t.Fatal(err)
	}

	certInfo2, _ := os.Stat(certFile)
	if !certInfo2.ModTime().Equal(origModTime) {
		t.Error("cert was regenerated despite matching domains and valid expiry")
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(f, []byte("yes"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !fileExists(f) {
		t.Error("expected true for existing file")
	}
	if fileExists(filepath.Join(dir, "nope.txt")) {
		t.Error("expected false for missing file")
	}
}

func TestCertNeedsRegenerationDomainChanged(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	// Generate cert with localhost only
	if err := generateSelfSigned(certFile, keyFile, []string{"localhost"}); err != nil {
		t.Fatal(err)
	}

	// Should need regen when domains change
	needsRegen, reason := certNeedsRegeneration(certFile, []string{"localhost", "api.app.localhost"})
	if !needsRegen {
		t.Error("expected regeneration when domains changed")
	}
	if reason == "" {
		t.Error("expected a reason for regeneration")
	}
}

func TestCertNeedsRegenerationSameDomains(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	domains := []string{"localhost", "127.0.0.1"}
	if err := generateSelfSigned(certFile, keyFile, domains); err != nil {
		t.Fatal(err)
	}

	needsRegen, _ := certNeedsRegeneration(certFile, domains)
	if needsRegen {
		t.Error("should not need regeneration with same domains")
	}
}

func TestCertNeedsRegenerationInvalidPEM(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	if err := os.WriteFile(certFile, []byte("not a cert"), 0o644); err != nil {
		t.Fatal(err)
	}

	needsRegen, reason := certNeedsRegeneration(certFile, []string{"localhost"})
	if !needsRegen {
		t.Error("expected regeneration for invalid PEM")
	}
	if reason != "invalid PEM data" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestCertNeedsRegenerationMissingFile(t *testing.T) {
	needsRegen, reason := certNeedsRegeneration("/nonexistent/cert.pem", []string{"localhost"})
	if !needsRegen {
		t.Error("expected regeneration for missing file")
	}
	if reason != "cannot read certificate" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestExtractDomains(t *testing.T) {
	compose := `services:
  api:
    image: api:latest
  web:
    image: web:latest
  postgres:
    image: postgres:16
`
	domains := ExtractDomains([]byte(compose), "app.localhost")

	domainSet := make(map[string]bool)
	for _, d := range domains {
		domainSet[d] = true
	}

	// Base domains always included
	if !domainSet["localhost"] {
		t.Error("expected localhost in domains")
	}
	if !domainSet["127.0.0.1"] {
		t.Error("expected 127.0.0.1 in domains")
	}
	if !domainSet["app.localhost"] {
		t.Error("expected app.localhost in domains")
	}

	// Service subdomains
	if !domainSet["api.app.localhost"] {
		t.Error("expected api.app.localhost in domains")
	}
	if !domainSet["web.app.localhost"] {
		t.Error("expected web.app.localhost in domains")
	}
	if !domainSet["postgres.app.localhost"] {
		t.Error("expected postgres.app.localhost in domains")
	}
}

func TestExtractDomainsInvalidYAML(t *testing.T) {
	domains := ExtractDomains([]byte("{{invalid"), "app.localhost")
	// Should still return base domains
	if len(domains) < 3 {
		t.Errorf("expected at least base domains, got %v", domains)
	}
}

func TestExtractDomainsNoServices(t *testing.T) {
	domains := ExtractDomains([]byte("version: '3'\n"), "app.localhost")
	domainSet := make(map[string]bool)
	for _, d := range domains {
		domainSet[d] = true
	}
	if !domainSet["localhost"] {
		t.Error("expected localhost even without services")
	}
	if !domainSet["app.localhost"] {
		t.Error("expected app.localhost even without services")
	}
}

func TestWriteAndReadDomainsList(t *testing.T) {
	dir := t.TempDir()
	domains := []string{"api.app.localhost", "app.localhost", "localhost", "web.app.localhost"}

	if err := WriteDomainsList(dir, domains); err != nil {
		t.Fatal(err)
	}

	read, err := ReadDomainsList(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(read) != len(domains) {
		t.Fatalf("expected %d domains, got %d", len(domains), len(read))
	}
	for i, d := range domains {
		if read[i] != d {
			t.Errorf("domain %d: expected %q, got %q", i, d, read[i])
		}
	}
}

func TestReadDomainsListMissing(t *testing.T) {
	_, err := ReadDomainsList("/nonexistent/dir")
	if err == nil {
		t.Error("expected error for missing domains file")
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]bool{"charlie": true, "alpha": true, "bravo": true}
	keys := sortedKeys(m)
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "alpha" || keys[1] != "bravo" || keys[2] != "charlie" {
		t.Errorf("expected sorted order, got %v", keys)
	}
}

func TestGenerateSelfSignedMultipleDomains(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	domains := []string{"localhost", "api.app.localhost", "web.app.localhost", "127.0.0.1", "::1"}
	if err := generateSelfSigned(certFile, keyFile, domains); err != nil {
		t.Fatal(err)
	}

	certData, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certData)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	// Check DNS names
	dnsSet := make(map[string]bool)
	for _, d := range cert.DNSNames {
		dnsSet[d] = true
	}
	if !dnsSet["localhost"] {
		t.Error("expected localhost in DNS names")
	}
	if !dnsSet["api.app.localhost"] {
		t.Error("expected api.app.localhost in DNS names")
	}

	// Check IP addresses
	if len(cert.IPAddresses) < 2 {
		t.Errorf("expected at least 2 IP addresses, got %d", len(cert.IPAddresses))
	}
}

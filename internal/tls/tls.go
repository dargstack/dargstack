package tls

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const renewalThreshold = 30 * 24 * time.Hour // 30 days before expiry

// EnsureCertificates checks for TLS certificates and generates/regenerates them
// if missing, domains don't match, or the certificate is close to expiry.
func EnsureCertificates(certDir string, domains []string) error {
	if err := os.MkdirAll(certDir, 0o755); err != nil {
		return fmt.Errorf("create certificates directory: %w", err)
	}

	certFile := filepath.Join(certDir, "localhost.pem")
	keyFile := filepath.Join(certDir, "localhost-key.pem")

	if fileExists(certFile) && fileExists(keyFile) {
		needsRegen, reason := certNeedsRegeneration(certFile, domains)
		if !needsRegen {
			return nil
		}
		fmt.Printf("Regenerating TLS certificate: %s\n", reason)
	}

	if hasMkcert() {
		return generateWithMkcert(certDir, domains)
	}

	fmt.Println("mkcert not found - generating self-signed certificate")
	fmt.Println("  Install mkcert for browser-trusted local certificates: https://github.com/FiloSottile/mkcert")
	return generateSelfSigned(certFile, keyFile, domains)
}

// certNeedsRegeneration checks if the certificate needs to be regenerated.
func certNeedsRegeneration(certFile string, expectedDomains []string) (needsRegen bool, reason string) {
	data, err := os.ReadFile(certFile)
	if err != nil {
		return true, "cannot read certificate"
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return true, "invalid PEM data"
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true, "cannot parse certificate"
	}

	// Check expiry
	if time.Until(cert.NotAfter) < renewalThreshold {
		return true, fmt.Sprintf("expires in %d days", int(time.Until(cert.NotAfter).Hours()/24))
	}

	// Check domain match
	certDomains := make(map[string]bool)
	for _, d := range cert.DNSNames {
		certDomains[d] = true
	}
	for _, ip := range cert.IPAddresses {
		certDomains[ip.String()] = true
	}

	expectedSet := make(map[string]bool)
	for _, d := range expectedDomains {
		expectedSet[d] = true
	}

	// Compare sets
	if len(certDomains) != len(expectedSet) {
		return true, "domain list changed"
	}
	for d := range expectedSet {
		if !certDomains[d] {
			return true, fmt.Sprintf("missing domain %q", d)
		}
	}

	return false, ""
}

// ExtractDomains extracts domains from compose data for TLS certificate generation.
// It includes base domains (localhost, 127.0.0.1, ::1, stackDomain) and adds
// each service name as a subdomain of stackDomain.
func ExtractDomains(composeData []byte, stackDomain string) []string {
	domainSet := make(map[string]bool)

	// Always include base domains
	domainSet["localhost"] = true
	domainSet["127.0.0.1"] = true
	domainSet["::1"] = true
	domainSet[stackDomain] = true

	// Parse compose and add service subdomains
	var doc map[string]interface{}
	if err := yaml.Unmarshal(composeData, &doc); err != nil {
		return sortedKeys(domainSet)
	}

	svcMap, ok := doc["services"].(map[string]interface{})
	if !ok {
		return sortedKeys(domainSet)
	}

	for name := range svcMap {
		subdomain := fmt.Sprintf("%s.%s", name, stackDomain)
		domainSet[subdomain] = true
	}

	return sortedKeys(domainSet)
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func hasMkcert() bool {
	_, err := exec.LookPath("mkcert")
	return err == nil
}

func generateWithMkcert(certDir string, domains []string) error {
	installCA := exec.Command("mkcert", "-install")
	installCA.Stdout = os.Stdout
	installCA.Stderr = os.Stderr
	if err := installCA.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: mkcert CA installation failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "  Certificates will be generated but may not be trusted by browsers.\n")
	}

	certFile := filepath.Join(certDir, "localhost.pem")
	keyFile := filepath.Join(certDir, "localhost-key.pem")

	args := []string{"-cert-file", certFile, "-key-file", keyFile}
	args = append(args, domains...)

	cmd := exec.Command("mkcert", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkcert: %w", err)
	}
	return nil
}

func generateSelfSigned(certFile, keyFile string, domains []string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial number: %w", err)
	}

	var dnsNames []string
	var ipAddresses []net.IP
	for _, d := range domains {
		if ip := net.ParseIP(d); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else {
			dnsNames = append(dnsNames, d)
		}
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{Organization: []string{"dargstack development"}},
		DNSNames:     dnsNames,
		IPAddresses:  ipAddresses,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}

	certOut, err := os.Create(certFile)
	if err != nil {
		return fmt.Errorf("create cert file: %w", err)
	}
	defer func() { _ = certOut.Close() }()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("write certificate: %w", err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}

	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	defer func() { _ = keyOut.Close() }()

	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// WriteDomainsList writes the expected domains list alongside the cert for tracking changes.
func WriteDomainsList(certDir string, domains []string) error {
	path := filepath.Join(certDir, ".domains")
	return os.WriteFile(path, []byte(strings.Join(domains, "\n")+"\n"), 0o644)
}

// ReadDomainsList reads the previously stored domains list.
func ReadDomainsList(certDir string) ([]string, error) {
	path := filepath.Join(certDir, ".domains")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var domains []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			domains = append(domains, line)
		}
	}
	return domains, scanner.Err()
}

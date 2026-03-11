package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dargstack/dargstack/internal/config"
	"github.com/dargstack/dargstack/internal/tls"
)

var certificatesCmd = &cobra.Command{
	Use:     "certify",
	Aliases: []string{"tls"},
	Short:   "Generate TLS certificates",
	Long: `Generate TLS certificates for development.

Creates self-signed certificates for localhost and all service subdomains.
Certificates are stored in artifacts/certificates and must be trusted in your browser or client.`,
	RunE: runGenerateCerts,
}

func runGenerateCerts(_ *cobra.Command, _ []string) error {
	var composeData []byte
	var err error

	if production {
		composeData, err = buildProductionCompose()
	} else {
		composeData, err = buildDevelopmentCompose()
	}
	if err != nil {
		return err
	}

	domains := tls.ExtractDomains(composeData, cfg.Production.Domain)
	domains = append(domains, cfg.Development.Domains...)

	certDir := config.CertificatesDir(stackDir)
	if err := tls.EnsureCertificates(certDir, domains); err != nil {
		return fmt.Errorf("TLS certificate generation failed: %w", err)
	}

	printSuccess(fmt.Sprintf("TLS certificates generated in %s for: %s", certDir, strings.Join(domains, ", ")))
	return nil
}

package lib

import (
	"crypto/x509"
	"fmt"
)

type ListenTLS struct {
	*ClientTLS

	AllowedDNSNames []string `long:"allowed-dns-name" description:"Allowed DNS names"`
}

func (l *ListenTLS) TLSConfig() error {
	if err := l.tlsConfig(false); err != nil {
		return err
	}

	if l.config != nil && len(l.CAFiles) > 0 && len(l.AllowedDNSNames) > 0 {
		l.config.VerifyPeerCertificate = l.verifyPeerCertificate
	}

	return nil
}

func (l *ListenTLS) verifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	cert, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return fmt.Errorf("caught error validating peer certificate: %v", err)
	}

	for _, name := range cert.DNSNames {
		for _, allowedName := range l.AllowedDNSNames {
			if name == allowedName {
				return nil
			}
		}
	}

	return fmt.Errorf("peer certificate not allowed because of DNS Name: %v", cert.DNSNames)
}

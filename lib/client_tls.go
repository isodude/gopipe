package lib

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

type ClientTLS struct {
	config *tls.Config

	CAFiles  []string `long:"ca-file" description:"TLS CA file"`
	CertFile string   `long:"cert-file" description:"TLS Cert file"`
	KeyFile  string   `long:"key-file" description:"TLS Key file"`
	Debug    bool     `long:"debug"`
}

func (c *ClientTLS) Args(group string) (args []string) {
	if c.CertFile != "" {
		args = append(args, fmt.Sprintf("--%s.cert-file=%s", group, c.CertFile))
	}
	if c.KeyFile != "" {
		args = append(args, fmt.Sprintf("--%s.key-file=%s", group, c.KeyFile))
	}
	if c.Debug {
		args = append(args, fmt.Sprintf("--%s.debug", group))
	}
	for _, k := range c.CAFiles {
		args = append(args, fmt.Sprintf("--%s.ca-file=%s", group, k))
	}

	return
}

func (c *ClientTLS) TLSConfig() error {
	return c.tlsConfig(true)
}

func (c *ClientTLS) tlsConfig(client bool) error {
	if len(c.CAFiles) == 0 && c.KeyFile == "" && c.CertFile == "" {
		return nil
	}

	c.config = &tls.Config{
		Renegotiation: tls.RenegotiateNever,
	}

	if len(c.CAFiles) > 0 {
		pool := x509.NewCertPool()
		for _, cert := range c.CAFiles {
			pem, err := os.ReadFile(cert)
			if err != nil {
				return fmt.Errorf(
					"could not read certificate %q: %v", cert, err)
			}
			if !pool.AppendCertsFromPEM(pem) {
				return fmt.Errorf(
					"could not parse any PEM certificates %q: %v", cert, err)
			}
		}
		if client {
			c.config.RootCAs = pool
		} else {
			c.config.ClientCAs = pool
			c.config.ClientAuth = tls.RequireAndVerifyClientCert
		}
	}

	if c.CertFile != "" && c.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return fmt.Errorf(
				"could not load keypair %s:%s: %v", c.CertFile, c.KeyFile, err)
		}

		c.config.Certificates = []tls.Certificate{cert}
	}

	return nil
}

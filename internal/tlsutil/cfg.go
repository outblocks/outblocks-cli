// Inspired by similar approach in: https://github.com/helm/helm (Apache 2.0 License).
package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"github.com/ansel1/merry/v2"
)

// Options represents configurable options used to create client and server TLS configurations.
type Options struct {
	CaCertFile string
	// If either the KeyFile or CertFile is empty, ClientConfig() will not load them.
	KeyFile  string
	CertFile string
	// Client-only options
	InsecureSkipVerify bool
}

// ClientConfig returns a TLS configuration for use by a Helm client.
func ClientConfig(opts Options) (cfg *tls.Config, err error) {
	var (
		cert *tls.Certificate
		pool *x509.CertPool
	)

	if opts.CertFile != "" || opts.KeyFile != "" {
		if cert, err = CertFromFilePair(opts.CertFile, opts.KeyFile); err != nil {
			if os.IsNotExist(err) {
				return nil, merry.Errorf("could not load x509 key pair (cert: %q, key: %q): %w", opts.CertFile, opts.KeyFile, err)
			}

			return nil, merry.Errorf("could not read x509 key pair (cert: %q, key: %q): %w", opts.CertFile, opts.KeyFile, err)
		}
	}

	if !opts.InsecureSkipVerify && opts.CaCertFile != "" {
		if pool, err = CertPoolFromFile(opts.CaCertFile); err != nil {
			return nil, err
		}
	}

	cfg = &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify, Certificates: []tls.Certificate{*cert}, RootCAs: pool}

	return cfg, nil
}

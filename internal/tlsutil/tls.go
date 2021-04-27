// Inspired by similar approach in: https://github.com/helm/helm (Apache 2.0 License).
package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
)

// NewClientTLS returns tls.Config appropriate for client auth.
func NewClientTLS(certFile, keyFile, caFile string) (*tls.Config, error) {
	config := tls.Config{}

	if certFile != "" && keyFile != "" {
		cert, err := CertFromFilePair(certFile, keyFile)
		if err != nil {
			return nil, err
		}

		config.Certificates = []tls.Certificate{*cert}
	}

	if caFile != "" {
		cp, err := CertPoolFromFile(caFile)
		if err != nil {
			return nil, err
		}

		config.RootCAs = cp
	}

	return &config, nil
}

// CertPoolFromFile returns an x509.CertPool containing the certificates
// in the given PEM-encoded file.
// Returns an error if the file could not be read, a certificate could not
// be parsed, or if the file does not contain any certificates.
func CertPoolFromFile(filename string) (*x509.CertPool, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("can't read CA file: %v", filename)
	}

	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(b) {
		return nil, fmt.Errorf("failed to append certificates from file: %s", filename)
	}

	return cp, nil
}

// CertFromFilePair returns an tls.Certificate containing the
// certificates public/private key pair from a pair of given PEM-encoded files.
// Returns an error if the file could not be read, a certificate could not
// be parsed, or if the file does not contain any certificates.
func CertFromFilePair(certFile, keyFile string) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("can't load key pair from cert %s and key %s: %w", certFile, keyFile, err)
	}

	return &cert, err
}

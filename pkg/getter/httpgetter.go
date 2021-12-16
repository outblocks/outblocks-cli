// Inspired by similar approach in: https://github.com/helm/helm (Apache 2.0 License).
package getter

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"

	"github.com/outblocks/outblocks-cli/internal/tlsutil"
	"github.com/outblocks/outblocks-cli/internal/urlutil"
	"github.com/outblocks/outblocks-cli/internal/version"
)

// HTTPGetter is the default HTTP(/S) backend handler.
type HTTPGetter struct {
	opts options
}

// Get performs a Get from repo.Getter and returns the body.
func (g *HTTPGetter) Get(ctx context.Context, href string, options ...Option) (*bytes.Buffer, error) {
	for _, opt := range options {
		opt(&g.opts)
	}

	return g.get(ctx, href)
}

func (g *HTTPGetter) get(ctx context.Context, href string) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)

	// Set a helm specific user agent so that a repo server and metrics can
	// separate helm calls from other tools interacting with repos.
	req, err := http.NewRequestWithContext(ctx, "GET", href, http.NoBody)
	if err != nil {
		return buf, err
	}

	req.Header.Set("User-Agent", version.UserAgent())

	if g.opts.userAgent != "" {
		req.Header.Set("User-Agent", g.opts.userAgent)
	}

	if g.opts.username != "" && g.opts.password != "" {
		req.SetBasicAuth(g.opts.username, g.opts.password)
	}

	client, err := g.httpClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return buf, err
	}

	if resp.StatusCode != 200 {
		return buf, fmt.Errorf("failed to fetch %s : %s", href, resp.Status)
	}

	_, err = io.Copy(buf, resp.Body)
	resp.Body.Close()

	return buf, err
}

// NewHTTPGetter constructs a valid http/https client as a Getter.
func NewHTTPGetter(options ...Option) (Getter, error) {
	var client HTTPGetter

	for _, opt := range options {
		opt(&client.opts)
	}

	return &client, nil
}

func (g *HTTPGetter) httpClient() (*http.Client, error) {
	transport := &http.Transport{
		DisableCompression: true,
		Proxy:              http.ProxyFromEnvironment,
	}

	if (g.opts.certFile != "" && g.opts.keyFile != "") || g.opts.caFile != "" {
		tlsConf, err := tlsutil.NewClientTLS(g.opts.certFile, g.opts.keyFile, g.opts.caFile)
		if err != nil {
			return nil, fmt.Errorf("can't create TLS config for client: %w", err)
		}

		tlsConf.BuildNameToCertificate()

		sni, err := urlutil.ExtractHostname(g.opts.url)
		if err != nil {
			return nil, err
		}

		tlsConf.ServerName = sni
		transport.TLSClientConfig = tlsConf
	}

	if g.opts.insecureSkipVerifyTLS {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		} else {
			transport.TLSClientConfig.InsecureSkipVerify = true
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   g.opts.timeout,
	}

	return client, nil
}

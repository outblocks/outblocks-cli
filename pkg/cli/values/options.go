// Inspired by similar approach in: https://github.com/helm/helm (Apache 2.0 License).
package values

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ansel1/merry/v2"
	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/pkg/getter"
	"github.com/outblocks/outblocks-cli/pkg/strvals"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

type Options struct {
	ValueFiles []string
	Values     []string
}

func (opts *Options) readFile(ctx context.Context, rootPath, filePath string, p getter.Providers) ([]byte, error) {
	bytes, err := readFile(ctx, filePath, p)
	if err != nil {
		var perr *os.PathError
		if errors.As(err, &perr) && !filepath.IsAbs(filePath) && rootPath != "" {
			// Try different cfg root path.
			filePath = filepath.Join(rootPath, filePath)
			return os.ReadFile(filePath)
		}
	}

	return bytes, err
}

// MergeValues merges values from files specified via -f/--values and directly
// via --set marshaling them to YAML.
func (opts *Options) MergeValues(ctx context.Context, root string, p getter.Providers) (map[string]interface{}, error) {
	base := map[string]interface{}{}

	// User specified a values files via -f/--values
	for _, filePath := range opts.ValueFiles {
		currentMap := map[string]interface{}{}

		bytes, err := opts.readFile(ctx, root, filePath, p)
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(bytes, &currentMap); err != nil {
			return nil, merry.Errorf("failed to parse %s: %w", filePath, err)
		}
		// Merge with the previous map
		base = plugin_util.MergeMaps(base, currentMap)
	}

	// User specified a value via --set
	for _, value := range opts.Values {
		if err := strvals.ParseInto(value, base); err != nil {
			return nil, merry.Errorf("failed parsing --set data: %w", err)
		}
	}

	return base, nil
}

// readFile load a file from stdin, the local directory, or a remote file with a url.
func readFile(ctx context.Context, filePath string, p getter.Providers) ([]byte, error) {
	if strings.TrimSpace(filePath) == "-" {
		return io.ReadAll(os.Stdin)
	}

	u, _ := url.Parse(filePath)

	g, err := p.ByScheme(u.Scheme)
	if err != nil {
		return os.ReadFile(filePath)
	}

	data, err := g.Get(ctx, filePath, getter.WithURL(filePath))

	return data.Bytes(), err
}

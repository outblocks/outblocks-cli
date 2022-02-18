package urlutil

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/Masterminds/semver"
)

func CheckLatestGitHubVersion(ctx context.Context, repoURL string) (*semver.Version, error) {
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(u.Path, "/releases/latest")

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	loc := resp.Header.Get("Location")
	idx := strings.LastIndex(loc, "/")

	if resp.StatusCode != 302 || idx == -1 {
		return nil, fmt.Errorf("failed to fetch latest release for %s : %s", repoURL, resp.Status)
	}

	resp.Body.Close()

	return semver.NewVersion(loc[idx+1:])
}

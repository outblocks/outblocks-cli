package plugins

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/google/go-github/v35/github"
	"github.com/mholt/archiver/v3"
	"github.com/outblocks/outblocks-cli/pkg/clipath"
	"github.com/outblocks/outblocks-cli/pkg/getter"
	"golang.org/x/oauth2"
)

type GitHubDownloader struct {
	client *github.Client
	vcs    *VCSDownloader
}

func NewGitHubDownloader() *GitHubDownloader {
	var tc *http.Client

	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)

		tc = oauth2.NewClient(context.Background(), ts)
	}

	return &GitHubDownloader{
		client: github.NewClient(tc),
		vcs:    NewVCSDownloader(),
	}
}

var GitHubRegex = regexp.MustCompile(`^https://github\.com/(?P<owner>[^/]+)/(?P<name>[^/]+)/?$`)

func (d *GitHubDownloader) Download(ctx context.Context, pi *pluginInfo) (*DownloadedPlugin, error) {
	matches := GitHubRegex.FindStringSubmatch(pi.source)
	repoOwner := matches[GitHubRegex.SubexpIndex("owner")]
	repoName := matches[GitHubRegex.SubexpIndex("name")]

	info, tag, err := d.vcs.download(ctx, pi)
	if err != nil {
		return nil, err
	}

	rel, _, err := d.client.Repositories.GetReleaseByTag(ctx, repoOwner, repoName, tag)
	if err != nil {
		return nil, err
	}

	var matchingAsset *github.ReleaseAsset

	arch := CurrentArch()

	for _, ass := range rel.Assets {
		n := ass.GetName()

		if strings.Contains(n, arch) &&
			(strings.HasSuffix(n, ".zip") || strings.HasSuffix(n, ".tar.gz") ||
				strings.HasSuffix(n, ".tar.bz") || strings.HasSuffix(n, ".tar") || strings.HasSuffix(n, ".rar")) {
			matchingAsset = ass
			break
		}
	}

	if matchingAsset == nil {
		// No matching release found, using repo as a whole.
		return info, err
	}

	dest := clipath.CachePath("plugin-release", pi.author, pi.name, filepath.Base(matchingAsset.GetName()))
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return nil, fmt.Errorf("failed to create path %s: %w", dest, err)
	}

	g, err := getter.NewHTTPGetter()
	if err != nil {
		return nil, fmt.Errorf("http getter init error: %w", err)
	}

	b, err := g.Get(ctx, matchingAsset.GetBrowserDownloadURL())
	if err != nil {
		return nil, fmt.Errorf("downloading file error: %w", err)
	}

	err = ioutil.WriteFile(dest, b.Bytes(), 0755)
	if err != nil {
		return nil, fmt.Errorf("writing downloaded file error: %w", err)
	}

	outDest := clipath.CachePath("plugin-release", pi.author, pi.name, "out")
	_ = os.RemoveAll(outDest)

	err = archiver.Unarchive(dest, outDest)
	if err != nil {
		return nil, fmt.Errorf("unarchiving error: %w", err)
	}

	err = os.RemoveAll(dest)
	if err != nil {
		return nil, fmt.Errorf("error removing archive file: %w", err)
	}

	return &DownloadedPlugin{
		Path:     outDest,
		PathTemp: true,
		Version:  info.Version,
	}, nil
}

func (d *GitHubDownloader) MatchingVersion(ctx context.Context, pi *pluginInfo) (matching, latest *semver.Version, err error) {
	matches := GitHubRegex.FindStringSubmatch(pi.source)
	repoOwner := matches[GitHubRegex.SubexpIndex("owner")]
	repoName := matches[GitHubRegex.SubexpIndex("name")]

	_, m, l, err := d.vcs.matchingVersion(ctx, pi, func(tag string) bool {
		_, resp, _ := d.client.Repositories.GetReleaseByTag(ctx, repoOwner, repoName, tag)

		return resp.StatusCode == 200
	})
	if err != nil {
		return nil, nil, err
	}

	return m.ver, l.ver, nil
}

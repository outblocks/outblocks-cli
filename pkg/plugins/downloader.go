package plugins

import (
	"context"

	"github.com/Masterminds/semver"
)

type Downloader interface {
	Download(ctx context.Context, pi *pluginInfo) (*DownloadedPlugin, error)
	MatchingVersion(ctx context.Context, pi *pluginInfo) (matching, latest *semver.Version, err error)
}

var (
	_ Downloader = (*VCSDownloader)(nil)
	_ Downloader = (*GitHubDownloader)(nil)
)

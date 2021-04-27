package plugins

import (
	"context"
)

type Downloader interface {
	Download(ctx context.Context, pi *pluginInfo) (*DownloadedPlugin, error)
}

var (
	_ Downloader = (*VCSDownloader)(nil)
	_ Downloader = (*GitHubDownloader)(nil)
)

package plugins

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/vcs"
	"github.com/blang/semver/v4"
	"github.com/outblocks/outblocks-cli/pkg/clipath"
)

type VCSDownloader struct {
}

func NewVCSDownloader() *VCSDownloader {
	return &VCSDownloader{}
}

type DownloadedPlugin struct {
	Path     string
	PathTemp bool
	Version  *semver.Version
}

type vcsVersionInfo struct {
	ver *semver.Version
	tag string
}

func (d *VCSDownloader) fetch(_ context.Context, pi *pluginInfo) (vcs.Repo, error) {
	cachePath := clipath.CachePath("plugins", pi.author, pi.name)
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create path %s: %w", cachePath, err)
	}

	repo, err := vcs.NewRepo(pi.source, cachePath)
	if err != nil {
		return nil, err
	}

	if repo.CheckLocal() {
		if err := repo.Update(); err != nil {
			return nil, fmt.Errorf("cannot update source repo: %w", err)
		}
	} else if err := repo.Get(); err != nil {
		return nil, fmt.Errorf("cannot find source repo: %w", err)
	}

	return repo, nil
}

func (d *VCSDownloader) download(ctx context.Context, pi *pluginInfo) (*DownloadedPlugin, string, error) {
	repo, ver, _, err := d.matchingVersion(ctx, pi)
	if err != nil {
		return nil, "", err
	}

	if err := repo.UpdateVersion(ver.tag); err != nil {
		return nil, "", fmt.Errorf("cannot update version of repo: %w", err)
	}

	return &DownloadedPlugin{
		Path:    clipath.CachePath("plugins", pi.author, pi.name),
		Version: ver.ver,
	}, ver.tag, nil
}

func (d *VCSDownloader) Download(ctx context.Context, pi *pluginInfo) (*DownloadedPlugin, error) {
	dp, _, err := d.download(ctx, pi)

	return dp, err
}

func (d *VCSDownloader) matchingVersion(ctx context.Context, pi *pluginInfo) (repo vcs.Repo, matching, latest *vcsVersionInfo, err error) {
	repo, err = d.fetch(ctx, pi)
	if err != nil {
		return nil, nil, nil, err
	}

	tags, err := repo.Tags()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cannot find repo or git error: %w", err)
	}

	matching = &vcsVersionInfo{}
	latest = &vcsVersionInfo{}

	for _, tag := range tags {
		version, err := semver.Parse(strings.TrimLeft(tag, "v"))
		if err != nil {
			continue
		}

		if latest.ver == nil || latest.ver.LT(version) {
			latest.ver = &version
			latest.tag = tag
		}

		match := pi.matches(&version, matching.ver)
		if match == noMatch {
			continue
		}

		matching.ver = &version
		matching.tag = tag

		if match == matchExact {
			break
		}
	}

	if matching.ver == nil {
		return nil, nil, nil, ErrPluginNoMatchingVersionFound
	}

	return repo, matching, latest, nil
}

func (d *VCSDownloader) MatchingVersion(ctx context.Context, pi *pluginInfo) (matching, latest *semver.Version, err error) {
	_, m, l, err := d.matchingVersion(ctx, pi)
	if err != nil {
		return nil, nil, err
	}

	return m.ver, l.ver, nil
}

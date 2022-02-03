package plugins

import (
	"context"

	"github.com/Masterminds/semver"
	"github.com/Masterminds/vcs"
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/clipath"
)

type VCSDownloader struct {
}

func NewVCSDownloader() *VCSDownloader {
	return &VCSDownloader{}
}

type DownloadedPlugin struct {
	Dir     string
	TempDir bool
	Version *semver.Version
}

type vcsVersionInfo struct {
	ver *semver.Version
	tag string
}

func (d *VCSDownloader) fetch(_ context.Context, pi *pluginInfo) (vcs.Repo, error) {
	cachePath := clipath.CacheDir("plugins", pi.author, pi.name)
	if err := fileutil.MkdirAll(cachePath, 0o755); err != nil {
		return nil, merry.Errorf("failed to create dir %s: %w", cachePath, err)
	}

	repo, err := vcs.NewRepo(pi.source, cachePath)
	if err != nil {
		return nil, err
	}

	if repo.CheckLocal() {
		if err := repo.Update(); err != nil {
			return nil, merry.Errorf("cannot update source repo: %w", err)
		}
	} else if err := repo.Get(); err != nil {
		return nil, merry.Errorf("cannot find source repo: %w", err)
	}

	return repo, fileutil.LchownRToUser(cachePath)
}

func (d *VCSDownloader) download(ctx context.Context, pi *pluginInfo) (*DownloadedPlugin, string, error) {
	repo, ver, _, err := d.matchingVersion(ctx, pi, nil)
	if err != nil {
		return nil, "", err
	}

	if err := repo.UpdateVersion(ver.tag); err != nil {
		return nil, "", merry.Errorf("cannot update version of repo: %w", err)
	}

	return &DownloadedPlugin{
		Dir:     clipath.CacheDir("plugins", pi.author, pi.name),
		Version: ver.ver,
	}, ver.tag, nil
}

func (d *VCSDownloader) Download(ctx context.Context, pi *pluginInfo) (*DownloadedPlugin, error) {
	dp, _, err := d.download(ctx, pi)

	return dp, err
}

func (d *VCSDownloader) matchingVersion(ctx context.Context, pi *pluginInfo, check func(tag string) bool) (repo vcs.Repo, matching, latest *vcsVersionInfo, err error) {
	repo, err = d.fetch(ctx, pi)
	if err != nil {
		return nil, nil, nil, err
	}

	tags, err := repo.Tags()
	if err != nil {
		return nil, nil, nil, merry.Errorf("cannot find repo or git error: %w", err)
	}

	checked := make(map[string]bool)

	for {
		matching = &vcsVersionInfo{}
		latest = &vcsVersionInfo{}

		for _, tag := range tags {
			version, err := semver.NewVersion(tag)
			if err != nil {
				continue
			}

			if v, ok := checked[tag]; ok && !v {
				continue
			}

			if latest.ver == nil || latest.ver.LessThan(version) {
				latest.ver = version
				latest.tag = tag
			}

			match := pi.matches(version, matching.ver)
			if match == noMatch {
				continue
			}

			matching.ver = version
			matching.tag = tag

			if match == matchExact {
				break
			}
		}

		if matching.ver == nil {
			return nil, nil, nil, ErrPluginNoMatchingVersionFound
		}

		if check == nil || (checked[matching.tag] && checked[latest.tag]) {
			return repo, matching, latest, nil
		}

		// Check if matching and latest tags are valid.
		if _, ok := checked[matching.tag]; !ok {
			checked[matching.tag] = check(matching.tag)
		}

		if _, ok := checked[latest.tag]; !ok {
			checked[latest.tag] = check(latest.tag)
		}

		if checked[matching.tag] && checked[latest.tag] {
			return repo, matching, latest, nil
		}
	}
}

func (d *VCSDownloader) MatchingVersion(ctx context.Context, pi *pluginInfo) (matching, latest *semver.Version, err error) {
	_, m, l, err := d.matchingVersion(ctx, pi, nil)
	if err != nil {
		return nil, nil, err
	}

	return m.ver, l.ver, nil
}

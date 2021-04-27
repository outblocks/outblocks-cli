package plugins

import (
	"context"
	"fmt"
	"os"

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
	Path    string
	Version *semver.Version
	Tag     string
}

func (d *VCSDownloader) Download(ctx context.Context, pi *pluginInfo) (*DownloadedPlugin, error) {
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
	} else if err := repo.Update(); err != nil {
		return nil, fmt.Errorf("cannot find source repo: %w", err)
	}

	tags, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("cannot find repo or git error: %w", err)
	}

	var (
		highestVer *semver.Version
		highestTag string
	)

	for _, tag := range tags {
		version, err := semver.Parse(tag)
		if err != nil {
			continue
		}

		match := pi.matches(&version, highestVer)
		if match == noMatch {
			continue
		}

		highestVer = &version
		highestTag = tag

		if match == matchExact {
			break
		}
	}

	if highestVer == nil {
		return nil, ErrPluginNoMatchingVersionFound
	}

	if err := repo.UpdateVersion(highestTag); err != nil {
		return nil, fmt.Errorf("cannot update version of repo: %w", err)
	}

	return &DownloadedPlugin{
		Path:    cachePath,
		Version: highestVer,
		Tag:     highestTag,
	}, nil
}

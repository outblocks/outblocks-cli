// Inspired by similar approach in: https://github.com/helm/helm (Apache 2.0 License).
package templating

import (
	"io/fs"
	"os"

	"github.com/Masterminds/vcs"
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/pkg/clipath"
)

type VCSInstaller struct {
	Repo    vcs.Repo
	Version string
	base
}

func NewVCSInstaller(source, version string) (*VCSInstaller, error) {
	key, err := cacheKey(source)
	if err != nil {
		return nil, err
	}

	cachedpath := clipath.CacheDir("templates", key)

	repo, err := vcs.NewRepo(source, cachedpath)
	if err != nil {
		return nil, err
	}

	i := &VCSInstaller{
		Repo:    repo,
		Version: version,
		base:    newBase(source, cachedpath),
	}

	i.filter = func(path string, entry fs.DirEntry) bool {
		return entry.Name() != ".git"
	}

	return i, err
}

func (i *VCSInstaller) Download() error {
	if _, err := os.Stat(i.Repo.LocalPath()); os.IsNotExist(err) {
		return i.Repo.Get()
	}

	if i.Repo.IsDirty() {
		return merry.Errorf("template cache repository '%s' was modified", i.Repo.LocalPath())
	}

	if err := i.Repo.Update(); err != nil {
		return err
	}

	if !isTemplate(i.Repo.LocalPath()) {
		return ErrMissingMetadata
	}

	return nil
}

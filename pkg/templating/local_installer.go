// Inspired by similar approach in: https://github.com/helm/helm (Apache 2.0 License).
package templating

import (
	"io/fs"
	"path/filepath"

	"github.com/ansel1/merry/v2"
)

type LocalInstaller struct {
	base
}

func NewLocalInstaller(source string) (*LocalInstaller, error) {
	src, err := filepath.Abs(source)
	if err != nil {
		return nil, merry.Errorf("unable to get absolute path to plugin: %w")
	}

	i := &LocalInstaller{
		base: newBase(source, src),
	}

	i.filter = func(path string, entry fs.DirEntry) bool {
		return entry.Name() != ".git"
	}

	return i, nil
}

func (i *LocalInstaller) Download() error {
	if !isTemplate(i.src) {
		return ErrMissingMetadata
	}

	return nil
}

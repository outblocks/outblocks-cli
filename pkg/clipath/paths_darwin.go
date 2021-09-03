//go:build darwin

package clipath

import (
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
)

func mustHomeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		panic("can't detect home directory")
	}

	return home
}

func dataHome() string {
	return filepath.Join(mustHomeDir(), "Library")
}

func configHome() string {
	return filepath.Join(mustHomeDir(), "Library", "Preferences")
}

func cacheHome() string {
	return filepath.Join(mustHomeDir(), "Library", "Caches")
}

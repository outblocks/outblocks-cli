// +build darwin

package clipath

import (
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
)

func mustHomedir() string {
	home, err := homedir.Dir()
	if err != nil {
		panic("can't detect home directory")
	}

	return home
}

func dataHome() string {
	return filepath.Join(mustHomedir(), "Library")
}

func configHome() string {
	return filepath.Join(mustHomedir(), "Library", "Preferences")
}

func cacheHome() string {
	return filepath.Join(mustHomedir(), "Library", "Caches")
}

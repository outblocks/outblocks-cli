package clipath

import (
	"os"
	"path/filepath"
)

const (
	// CacheHomeEnvVar is the environment variable used by Outblocks
	// for the cache directory. When no value is set a default is used.
	CacheHomeEnvVar = "OUTBLOCKS_CACHE_HOME"

	// ConfigHomeEnvVar is the environment variable used by Outblocks
	// for the config directory. When no value is set a default is used.
	ConfigHomeEnvVar = "OUTBLOCKS_CONFIG_HOME"

	// DataHomeEnvVar is the environment variable used by Outblocks
	// for the data directory. When no value is set a default is used.
	DataHomeEnvVar = "OUTBLOCKS_DATA_HOME"
)

// lazydir is an lazy-loaded dir buffer for the XDG base directory specification.
type lazydir string

func (l lazydir) dir(envVar string, defaultFn func() string, elem ...string) string {
	// There is an order to checking for a dir.
	// 1. See if a Outblocks environment variable has been set.
	// 2. Fall back to a default
	base := os.Getenv(envVar)
	if base != "" {
		return filepath.Join(base, filepath.Join(elem...))
	}

	return filepath.Join(defaultFn(), string(l), filepath.Join(elem...))
}

// cacheDir defines the base directory relative to which user specific non-essential data files
// should be stored.
func (l lazydir) cacheDir(elem ...string) string {
	return l.dir(CacheHomeEnvVar, cacheHome, filepath.Join(elem...))
}

// configDir defines the base directory relative to which user specific configuration files should
// be stored.
func (l lazydir) configDir(elem ...string) string {
	return l.dir(ConfigHomeEnvVar, configHome, filepath.Join(elem...))
}

// dataDir defines the base directory relative to which user specific data files should be stored.
func (l lazydir) dataDir(elem ...string) string {
	return l.dir(DataHomeEnvVar, dataHome, filepath.Join(elem...))
}

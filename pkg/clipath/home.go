// Inspired by similar approach in: https://github.com/helm/helm (Apache 2.0 License).
package clipath

const lp = lazypath("outblocks")

// ConfigPath returns the path where Outblocks stores configuration.
func ConfigPath(elem ...string) string { return lp.configPath(elem...) }

// CachePath returns the path where Outblocks stores cached objects.
func CachePath(elem ...string) string { return lp.cachePath(elem...) }

// DataPath returns the path where Outblocks stores data.
func DataPath(elem ...string) string { return lp.dataPath(elem...) }

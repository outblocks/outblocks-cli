// Inspired by similar approach in: https://github.com/helm/helm (Apache 2.0 License).
package clipath

const lp = lazydir("outblocks")

// ConfigDir returns the dir where Outblocks stores configuration.
func ConfigDir(elem ...string) string { return lp.configDir(elem...) }

// CacheDir returns the dir where Outblocks stores cached objects.
func CacheDir(elem ...string) string { return lp.cacheDir(elem...) }

// DataDir returns the dir where Outblocks stores data.
func DataDir(elem ...string) string { return lp.dataDir(elem...) }

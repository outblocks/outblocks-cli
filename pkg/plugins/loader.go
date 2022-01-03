package plugins

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/ansel1/merry/v2"
	"github.com/goccy/go-yaml"
	"github.com/otiai10/copy"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
)

const (
	DefaultPluginSourceFmt = "https://github.com/outblocks/cli-plugin-%s"
	DefaultAuthor          = "outblocks"
	A                      = runtime.GOOS + "_" + runtime.GOARCH
)

func CurrentArch() string {
	if runtime.GOARCH == "arm" {
		return runtime.GOOS + "_armv6"
	}

	return runtime.GOOS + "_" + runtime.GOARCH
}

type Loader struct {
	baseDir, pluginsCacheDir string

	downloader struct {
		vcs    *VCSDownloader
		github *GitHubDownloader
	}
}

func NewLoader(baseDir, pluginsCacheDir string) *Loader {
	l := &Loader{
		baseDir:         baseDir,
		pluginsCacheDir: pluginsCacheDir,
	}

	l.downloader.github = NewGitHubDownloader()
	l.downloader.vcs = NewVCSDownloader()

	return l
}

func (l *Loader) LoadPlugin(ctx context.Context, name, src string, verConstr *semver.Constraints, lock *lockfile.Plugin) (*Plugin, error) {
	pi := newPluginInfo(name, src, verConstr, lock)

	path, ver := l.findInstalledPluginLocation(pi)

	if path == "" {
		var err error

		path, ver, err = l.installCachedPlugin(pi)
		if err != nil {
			return nil, err
		}
	}

	if path == "" {
		return nil, ErrPluginNotFound
	}

	return l.loadPlugin(pi, path, ver)
}

func (l *Loader) DownloadPlugin(ctx context.Context, name string, verConstr *semver.Constraints, src string, lock *lockfile.Plugin) (*Plugin, error) {
	pi := newPluginInfo(name, src, verConstr, lock)

	from, ver, err := l.downloadPlugin(ctx, pi)
	if err != nil {
		return nil, err
	}

	if err := l.installPlugin(pi, from); err != nil {
		return nil, err
	}

	return l.loadPlugin(pi, from, ver)
}

func (l *Loader) findMatchingPluginLocation(pi *pluginInfo, path string) (string, *semver.Version) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", nil
	}

	var (
		highestVer *semver.Version
		pluginPath string
	)

	arch := CurrentArch()

	for _, entry := range entries {
		isSymlink := entry.Type()&fs.ModeSymlink != 0

		if !isSymlink && !entry.IsDir() {
			continue
		}

		parts := strings.SplitN(entry.Name(), "-", 2)
		if len(parts) != 2 || parts[0] != arch {
			continue
		}

		version, err := semver.NewVersion(parts[1])
		if err != nil {
			continue
		}

		match := pi.matches(version, highestVer)
		if match == noMatch {
			continue
		}

		dest := filepath.Join(path, entry.Name())

		if isSymlink {
			dest, err = filepath.EvalSymlinks(dest)
			if err != nil {
				continue
			}
		}

		pluginPath = dest
		highestVer = version

		if match == matchExact {
			break
		}
	}

	return pluginPath, highestVer
}

func (l *Loader) findCachedPluginLocation(pi *pluginInfo) (string, *semver.Version) {
	path := filepath.Join(l.pluginsCacheDir, pi.author, pi.name)

	return l.findMatchingPluginLocation(pi, path)
}

func (l *Loader) findInstalledPluginLocation(pi *pluginInfo) (string, *semver.Version) {
	path := filepath.Join(l.baseDir, ".outblocks", "plugins", pi.author, pi.name)

	return l.findMatchingPluginLocation(pi, path)
}

func (l *Loader) selectDownloader(src string) Downloader {
	if GitHubRegex.MatchString(src) {
		return l.downloader.github
	}

	return l.downloader.vcs
}

func (l *Loader) downloadPlugin(ctx context.Context, pi *pluginInfo) (string, *semver.Version, error) {
	download, err := l.selectDownloader(pi.source).Download(ctx, pi)
	if err != nil {
		return "", nil, merry.Errorf("failed to download plugin %s: %w", pi.name, err)
	}

	destDir := filepath.Join(l.pluginsCacheDir, pi.author, pi.name, fmt.Sprintf("%s-%s", CurrentArch(), download.Version))

	if err := fileutil.MkdirAll(destDir, 0755); err != nil {
		return "", nil, merry.Errorf("failed to create dir %s: %w", destDir, err)
	}

	if err := copy.Copy(download.Dir, destDir); err != nil {
		return "", nil, merry.Errorf("failed to copy downloaded plugin %s: %w", destDir, err)
	}

	if err := fileutil.LchownRToUser(destDir); err != nil {
		return "", nil, merry.Errorf("failed to set permissions on downloaded plugin %s: %w", destDir, err)
	}

	if download.TempDir {
		err = os.RemoveAll(download.Dir)
		if err != nil {
			return "", nil, merry.Errorf("failed to remove downloaded plugin temp dir %s: %w", download.Dir, err)
		}
	}

	return destDir, download.Version, nil
}

func (l *Loader) installCachedPlugin(pi *pluginInfo) (string, *semver.Version, error) {
	from, ver := l.findCachedPluginLocation(pi)

	if from == "" {
		return "", nil, ErrPluginNotFound
	}

	if err := l.installPlugin(pi, from); err != nil {
		return "", nil, err
	}

	return from, ver, nil
}

func (l *Loader) installPlugin(pi *pluginInfo, from string) error {
	localDir := filepath.Join(l.baseDir, ".outblocks", "plugins", pi.author, pi.name)
	if err := fileutil.MkdirAll(localDir, 0755); err != nil {
		return merry.Errorf("failed to create dir %s: %w", localDir, err)
	}

	dest := filepath.Join(localDir, filepath.Base(from))
	_ = os.RemoveAll(dest)

	if err := fileutil.Symlink(from, dest); err != nil {
		if err := copy.Copy(from, dest); err != nil {
			return merry.Errorf("failed to copy cached plugin %s: %w", from, err)
		}
	}

	return fileutil.LchownRToUser(dest)
}

func (l *Loader) loadPlugin(pi *pluginInfo, dir string, ver *semver.Version) (*Plugin, error) {
	p := fileutil.FindYAML(filepath.Join(dir, "plugin"))
	if p == "" {
		return nil, merry.Errorf("plugin yaml is missing in: %s", dir)
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return nil, merry.Errorf("cannot read yaml: %w", err)
	}

	cacheDir := filepath.Join(l.baseDir, ".outblocks", "cache", pi.name)

	err = fileutil.MkdirAll(cacheDir, 0755)
	if err != nil {
		return nil, merry.Errorf("failed to create dir %s: %w", cacheDir, err)
	}

	plugin := Plugin{
		Dir:      dir,
		CacheDir: cacheDir,
		Version:  ver,
		yamlData: data,
		yamlPath: p,
		source:   pi.source,
	}

	if err := yaml.UnmarshalWithOptions(data, &plugin, yaml.Validator(validator.DefaultValidator()), yaml.UseJSONUnmarshaler(), yaml.DisallowDuplicateKey()); err != nil {
		return nil, merry.Errorf("plugin config load failed.\nfile: %s\n%s", p, err)
	}

	return &plugin, nil
}

func (l *Loader) MatchingVersion(ctx context.Context, name, src string, verConstr *semver.Constraints) (matching, latest *semver.Version, err error) {
	pi := newPluginInfo(name, src, verConstr, nil)

	return l.selectDownloader(pi.source).MatchingVersion(ctx, pi)
}

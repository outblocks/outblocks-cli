// Inspired by similar approach in: https://github.com/helm/helm (Apache 2.0 License).
package templating

import (
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
)

type Installer interface {
	Download() error
	CopyTo(dest string) error
}

func NewInstaller(source, version string) (Installer, error) {
	if isLocalReference(source) {
		return NewLocalInstaller(source)
	}

	return NewVCSInstaller(source, version)
}

type base struct {
	src       string
	localPath string
	filter    func(path string, entry fs.DirEntry) bool
}

func newBase(source, localPath string) base {
	return base{
		src:       source,
		localPath: localPath,
	}
}

func (b *base) CopyTo(dest string) error {
	return fileutil.CopyDir(b.localPath, dest, b.filter)
}

func isTemplate(dirname string) bool {
	return fileutil.FindYAML(filepath.Join(dirname, TemplateYAMLName)) != ""
}

// scpSyntaxRe matches the SCP-like addresses used to access repos over SSH.
var scpSyntaxRe = regexp.MustCompile(`^([a-zA-Z0-9_]+)@([a-zA-Z0-9._-]+):(.*)$`)

// cacheKey generates a cache key based on a url or scp string. The key is file
// system safe.
func cacheKey(repo string) (string, error) {
	var (
		u   *url.URL
		err error
	)

	if m := scpSyntaxRe.FindStringSubmatch(repo); m != nil {
		// Match SCP-like syntax and convert it to a URL.
		// Eg, "git@github.com:user/repo" becomes
		// "ssh://git@github.com/user/repo".
		u = &url.URL{
			User: url.User(m[1]),
			Host: m[2],
			Path: "/" + m[3],
		}
	} else {
		u, err = url.Parse(repo)
		if err != nil {
			return "", err
		}
	}

	var key strings.Builder
	if u.Scheme != "" {
		key.WriteString(u.Scheme)
		key.WriteString("-")
	}

	if u.User != nil && u.User.Username() != "" {
		key.WriteString(u.User.Username())
		key.WriteString("-")
	}

	key.WriteString(u.Host)

	if u.Path != "" {
		key.WriteString(strings.ReplaceAll(u.Path, "/", "-"))
	}

	return strings.ReplaceAll(key.String(), ":", "-"), nil
}

func isLocalReference(source string) bool {
	_, err := os.Stat(source)
	return err == nil
}

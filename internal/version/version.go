// Inspired by similar approach in: https://github.com/helm/helm (Apache 2.0 License).
package version

import (
	"context"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/urlutil"
)

var (
	// Populated by goreleaser during build.
	version   = "0.0.0-snapshot"
	gitCommit = "xxx"
	date      = ""

	versionSemver *semver.Version
)

const (
	RepoURL                 = "https://github.com/outblocks/outblocks-cli"
	autoCheckInterval       = 2 * time.Hour
	LastUpdateCheckFileName = "last_update_check"
)

// BuildInfo describes the compile time information.
type BuildInfo struct {
	// Version is the current semver.
	Version string `json:"version,omitempty"`
	// Build date.
	Date string `json:"date,omitempty"`
	// GitCommit is the git sha1.
	GitCommit string `json:"git_commit,omitempty"`
	// GoVersion is the version of the Go compiler used.
	GoVersion string `json:"go_version,omitempty"`
}

// Version returns the semver string of the version.
func Version() string {
	return version
}

func Semver() *semver.Version {
	if versionSemver == nil {
		versionSemver = semver.MustParse(version)
	}

	return versionSemver
}

func Date() string {
	return date
}

// UserAgent returns a user agent for user with an HTTP client.
func UserAgent() string {
	return "Outblocks/" + strings.TrimPrefix(Version(), "v")
}

// Get returns build info.
func Get() BuildInfo {
	v := BuildInfo{
		Date:      Date(),
		Version:   Version(),
		GitCommit: gitCommit,
		GoVersion: runtime.Version(),
	}

	return v
}

func ShouldRunUpdateCheck(f string) bool {
	lastUpdateTime := fileutil.GetModTimeFromFile(f)
	return time.Since(lastUpdateTime) >= autoCheckInterval && Semver().Prerelease() == ""
}

func CheckLatestCLI(ctx context.Context) (*semver.Version, error) {
	return urlutil.CheckLatestGitHubVersion(ctx, RepoURL)
}

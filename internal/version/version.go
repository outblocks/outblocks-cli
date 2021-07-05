// Inspired by similar approach in: https://github.com/helm/helm (Apache 2.0 License).
package version

import (
	"runtime"
	"strings"
)

var (
	// Populated by goreleaser during build.
	version   = "snapshot"
	gitCommit = "xxx"
	date      = ""
)

// BuildInfo describes the compile time information.
type BuildInfo struct {
	// Version is the current semver.
	Version string `json:"version,omitempty"`
	// GitCommit is the git sha1.
	GitCommit string `json:"git_commit,omitempty"`
	// GitTreeState is the state of the git tree.
	GitTreeState string `json:"git_tree_state,omitempty"`
	// GoVersion is the version of the Go compiler used.
	GoVersion string `json:"go_version,omitempty"`
}

// Version returns the semver string of the version.
func Version() string {
	return version
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
		Version:   Version(),
		GitCommit: gitCommit,
		GoVersion: runtime.Version(),
	}

	return v
}

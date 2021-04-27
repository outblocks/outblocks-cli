package plugins

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
)

type pluginInfo struct {
	name, author, source string
	verRange             semver.Range
	lock                 *lockfile.Plugin
}

type matchType int

const (
	matchExact matchType = iota
	matchRange
	noMatch
)

func source(name, s string) string {
	if s != "" {
		return s
	}

	return fmt.Sprintf(DefaultPluginSourceFmt, name)
}

func author(name string) (a, n string) {
	s := strings.Split(name, "/")
	if len(s) == 2 {
		return s[0], s[1]
	}

	return DefaultAuthor, name
}

func (pi *pluginInfo) matches(version, highestVer *semver.Version) matchType {
	if pi.lock != nil {
		if pi.lock.Matches(pi.name, version, pi.source) {
			return matchExact
		}

		return noMatch
	}

	if pi.verRange != nil && pi.verRange(*version) && (highestVer == nil || highestVer.LT(*version)) {
		return matchRange
	}

	return noMatch
}

func newPluginInfo(name, src string, verRange semver.Range, lock *lockfile.Plugin) *pluginInfo {
	author, name := author(name)
	src = source(name, src)

	// If there is both a lock version and verRange but lock version doesn't match verRange, ignore it.
	if lock != nil && verRange != nil && !verRange(*lock.Version.Version) {
		lock = nil
	}

	return &pluginInfo{
		author:   author,
		name:     name,
		source:   src,
		verRange: verRange,
		lock:     lock,
	}
}

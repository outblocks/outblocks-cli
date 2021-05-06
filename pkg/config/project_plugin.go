package config

import (
	"github.com/blang/semver/v4"
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type ProjectPlugin struct {
	Name    string                 `json:"name"`
	Version string                 `json:"version"`
	Source  string                 `json:"source"`
	Other   map[string]interface{} `yaml:"-,remain"`

	verRange semver.Range
	order    uint
}

func (p *ProjectPlugin) VerRange() semver.Range {
	return p.verRange
}

func (p *ProjectPlugin) Order() uint {
	return p.order
}

func (p *ProjectConfig) SetPlugins(plugs []*plugins.Plugin) {
	p.plugins = plugs
}

func (p *ProjectConfig) PluginLock(plug *ProjectPlugin) *lockfile.Plugin {
	return p.lock.PluginByName(plug.Name)
}

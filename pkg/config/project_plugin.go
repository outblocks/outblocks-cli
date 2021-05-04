package config

import (
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type ProjectPlugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"`

	verRange semver.Range
	order    uint
}

func (p *ProjectPlugin) VerRange() semver.Range {
	return p.verRange
}

func (p *ProjectPlugin) Order() uint {
	return p.order
}

func (p *ProjectConfig) LoadPlugins(plugs []*plugins.Plugin) error {
	p.plugins = plugs

	for _, plug := range p.plugins {
		if err := plugins.NewPluginNormalizer(plug).Normalize(); err != nil {
			return err
		}
	}

	// Start plugins.
	for _, plug := range p.plugins {
		if err := plug.Start(p.Path); err != nil {
			return fmt.Errorf("error starting plugin '%s': %w", plug.Name, err)
		}
	}

	return nil
}

func (p *ProjectConfig) PluginLock(plug *ProjectPlugin) *lockfile.Plugin {
	return p.lock.PluginByName(plug.Name)
}

package config

import (
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
)

func (p *Project) Lockfile() *lockfile.Lockfile {
	plugins := make([]*lockfile.Plugin, len(p.plugins))
	for i, plug := range p.plugins {
		plugins[i] = plug.Locked()
	}

	return &lockfile.Lockfile{
		Plugins: plugins,
	}
}

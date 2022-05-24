package config

import (
	"fmt"
	"net/url"

	"github.com/Masterminds/semver"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type Plugin struct {
	Name    string                 `json:"name"`
	Short   string                 `json:"short"`
	Version string                 `json:"version"`
	Source  string                 `json:"source,omitempty"`
	Other   map[string]interface{} `yaml:"-,remain"`

	verConstr *semver.Constraints
	loaded    *plugins.Plugin
	order     uint
}

func (p *Plugin) SetLoaded(plug *plugins.Plugin) {
	p.loaded = plug
}

func (p *Plugin) Loaded() *plugins.Plugin {
	return p.loaded
}

func (p *Plugin) VerConstr() *semver.Constraints {
	return p.verConstr
}

func (p *Plugin) Order() uint {
	return p.order
}

func (p *Plugin) ShortName() string {
	if p.Short != "" {
		return p.Short
	}

	return p.Name
}

func (p *Plugin) Normalize(i int, cfg *Project) error {
	var err error

	if p.Version != "" {
		p.verConstr, err = semver.NewConstraint(p.Version)
		if err != nil {
			return cfg.yamlError(fmt.Sprintf("$.plugins[%d].version", i), "Plugin.version is in invalid format")
		}
	}

	if p.Source != "" {
		u, err := url.Parse(p.Source)
		if err != nil {
			return cfg.yamlError(fmt.Sprintf("$.plugins[%d].source", i), "Plugin.source is not a valid URL")
		}

		p.Source = u.String()
	}

	p.order = uint(i)

	return nil
}

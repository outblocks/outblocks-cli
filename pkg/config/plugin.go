package config

import (
	"fmt"
	"net/url"

	"github.com/blang/semver/v4"
)

type Plugin struct {
	Name    string                 `json:"name"`
	Version string                 `json:"version"`
	Source  string                 `json:"source"`
	Other   map[string]interface{} `yaml:"-,remain"`

	verRange semver.Range
	order    uint
}

func (p *Plugin) VerRange() semver.Range {
	return p.verRange
}

func (p *Plugin) Order() uint {
	return p.order
}

func (p *Plugin) Normalize(i int, cfg *ProjectConfig) error {
	var err error

	if p.Version != "" {
		p.verRange, err = semver.ParseRange(p.Version)
		if err != nil {
			return cfg.yamlError(fmt.Sprintf("$.plugins[%d].version", i), "p.version is in invalid format")
		}
	}

	if p.Source != "" {
		u, err := url.Parse(p.Source)
		if err != nil {
			return cfg.yamlError(fmt.Sprintf("$.plugins[%d].source", i), "p.source is not a valid URL")
		}

		p.Source = u.String()
	}

	p.order = uint(i)

	return nil
}

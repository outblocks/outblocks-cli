package config

import (
	"fmt"
	"strings"

	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type DNS struct {
	Domain string `json:"domain"`
	Plugin string `json:"plugin"`

	plugin *plugins.Plugin
}

func (s *DNS) Normalize(i int, cfg *ProjectConfig) error {
	s.Domain = strings.ToLower(s.Domain)

	if s.Domain == "" {
		return cfg.yamlError(fmt.Sprintf("$.dns[%d].domain", i), "state has no type defined, did you want \"type: local\"?")
	}

	return nil
}

func (s *DNS) Check(i int, cfg *ProjectConfig) error {
	if s.Plugin != "" {
		s.plugin = cfg.FindLoadedPlugin(s.Plugin)
	} else {
		for _, plug := range cfg.LoadedPlugins() {
			if plug.HasAction(plugins.ActionDNS) {
				s.plugin = plug

				break
			}
		}
	}

	if s.plugin == nil {
		return cfg.yamlError("$.dns[%d].domain", "dns has no supported plugin available")
	}

	return nil
}

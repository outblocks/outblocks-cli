package config

import (
	"strings"

	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type Secrets struct {
	Type  string                 `json:"type"`
	Other map[string]interface{} `yaml:"-,remain"`

	plugin *plugins.Plugin
}

func (s *Secrets) Normalize(cfg *Project) error {
	s.Type = strings.ToLower(s.Type)

	return nil
}

func (s *Secrets) Check(cfg *Project) error {
	// Check plugin.
	for _, plug := range cfg.loadedPlugins {
		if !plug.HasAction(plugins.ActionSecrets) {
			continue
		}

		for _, typ := range plug.SecretsTypes {
			if typ == s.Type || s.Type == "" {
				s.plugin = plug
			}
		}
	}

	return nil
}

func (s *Secrets) Plugin() *plugins.Plugin {
	return s.plugin
}

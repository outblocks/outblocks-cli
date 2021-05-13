package config

import (
	"strings"

	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

const (
	StateLocal = "local"
)

type State struct {
	Type  string                 `json:"type"`
	Other map[string]interface{} `yaml:"-,remain"`

	plugin *plugins.Plugin
}

func (s *State) Normalize(cfg *ProjectConfig) error {
	s.Type = strings.ToLower(s.Type)

	if s.Type == "" {
		return cfg.yamlError("$.state.type", "state has no type defined, did you want \"type: local\"?")
	}

	return nil
}

func (s *State) Check(cfg *ProjectConfig) error {
	if s.Type == StateLocal {
		return nil
	}

	// Check plugin.
	for _, plug := range cfg.plugins {
		for _, typ := range plug.StateTypes {
			if typ == s.Type {
				s.plugin = plug
			}
		}
	}

	if s.plugin == nil {
		return cfg.yamlError("$.state", "state has no supported plugin available")
	}

	return nil
}

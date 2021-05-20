package config

import (
	"strings"

	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

const (
	StateLocal      = "local"
	StateDefaultEnv = "dev"
)

type State struct {
	Type  string                 `json:"type"`
	Env   string                 `json:"env"`
	Other map[string]interface{} `yaml:"-,remain"`

	plugin *plugins.Plugin
}

func (s *State) Normalize(cfg *Project) error {
	s.Type = strings.ToLower(s.Type)

	if s.Type == "" {
		return cfg.yamlError("$.state.type", "state has no type defined, did you want \"type: local\"?")
	}

	s.Env = strings.ToLower(s.Env)

	if s.Env == "" {
		s.Env = StateDefaultEnv
	}

	return nil
}

func (s *State) Check(cfg *Project) error {
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

func (s *State) Plugin() *plugins.Plugin {
	return s.plugin
}

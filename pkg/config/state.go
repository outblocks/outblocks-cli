package config

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-plugin-go/types"
)

const (
	StateLocal      = "local"
	StateDefaultEnv = "dev"
	StateLocalPath  = "outblocks.state"
)

type State struct {
	Type  string                 `json:"type"`
	Env   string                 `json:"env"`
	Other map[string]interface{} `yaml:"-,remain"`

	plugin *plugins.Plugin
}

func (s *State) IsLocal() bool {
	return s.Type == StateLocal
}

func (s *State) LocalPath() string {
	p, ok := s.Other["path"]
	if !ok {
		return StateLocalPath
	}

	v, ok := p.(string)
	if !ok {
		return StateLocalPath
	}

	return v
}

func (s *State) LoadLocal() (*types.StateData, error) {
	data, err := ioutil.ReadFile(s.LocalPath())
	if err != nil {
		return nil, err
	}

	d := &types.StateData{}
	err = json.Unmarshal(data, &d)

	return d, err
}

func (s *State) SaveLocal(d *types.StateData) error {
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(s.LocalPath(), data, 0644)
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

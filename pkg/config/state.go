package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-plugin-go/types"
)

const (
	StateLocal     = "local"
	StateLocalPath = ".outblocks.state"
)

type State struct {
	Type  string                 `json:"type"`
	Path  string                 `json:"path"`
	Other map[string]interface{} `yaml:"-,remain"`

	env    string
	plugin *plugins.Plugin
}

func (s *State) IsLocal() bool {
	return s.Type == StateLocal
}

func (s *State) LocalPath() string {
	if s.Path == "" {
		return s.env + StateLocalPath
	}

	return s.Path
}

func (s *State) LoadLocal() (*types.StateData, error) {
	data, err := os.ReadFile(s.LocalPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &types.StateData{}, nil
		}

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

	return fileutil.WriteFile(s.LocalPath(), data, 0644)
}

func (s *State) Normalize(cfg *Project) error {
	s.Type = strings.ToLower(s.Type)

	if s.Type == "" {
		return cfg.yamlError("$.state.type", "state has no type defined, did you want \"type: local\"?")
	}

	return nil
}

func (s *State) Check(cfg *Project) error {
	if s.Type == StateLocal {
		return nil
	}

	// Check plugin.
	for _, plug := range cfg.loadedPlugins {
		if !plug.HasAction(plugins.ActionState) {
			continue
		}

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

func (s *State) Env() string {
	return s.env
}

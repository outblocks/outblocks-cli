package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/statefile"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
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

func (s *State) LoadLocal() (*statefile.StateData, error) {
	data, err := os.ReadFile(s.LocalPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return statefile.NewStateData(), nil
		}

		return nil, err
	}

	d := statefile.NewStateData()
	err = json.Unmarshal(data, &d)

	return d, err
}

func (s *State) SaveLocal(d *statefile.StateData) error {
	data, err := json.Marshal(d)
	if err != nil {
		return merry.Errorf("error marshaling state: %w", err)
	}

	return fileutil.WriteFile(s.LocalPath(), data, 0o644)
}

func (s *State) Normalize(cfg *Project) error {
	s.Type = strings.ToLower(s.Type)

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
			if typ == s.Type || s.Type == "" {
				s.plugin = plug
			}
		}
	}

	if s.plugin == nil {
		if s.Type == "" {
			return cfg.yamlError("$.state", "state has no type defined, did you want \"type: local\"?")
		}

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

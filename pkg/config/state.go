package config

import (
	"encoding/json"
	"io/ioutil"
	"regexp"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-plugin-go/types"
)

const (
	StateLocal      = "local"
	StateDefaultEnv = "dev"
	StateLocalPath  = ".outblocks.state"
)

type State struct {
	Type  string                 `json:"type"`
	Env   string                 `json:"env"`
	Path  string                 `json:"path"`
	Other map[string]interface{} `yaml:"-,remain"`

	plugin *plugins.Plugin
}

func (s *State) Validate() error {
	return validation.ValidateStruct(s,
		validation.Field(&s.Env, validation.Required, validation.Match(regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,20}$`))),
	)
}

func (s *State) IsLocal() bool {
	return s.Type == StateLocal
}

func (s *State) LocalPath() string {
	if s.Path == "" {
		return s.Env + StateLocalPath
	}

	return s.Path
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
	for _, plug := range cfg.loadedPlugins {
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

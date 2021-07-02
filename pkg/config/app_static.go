package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"github.com/outblocks/outblocks-plugin-go/types"
)

const (
	TypeStatic = "static"

	StaticAppRoutingReact = "react"

	DefaultStaticAppBuildDir = "build"
)

var StaticAppRoutings = []string{StaticAppRoutingReact}

type StaticApp struct {
	BasicApp `json:",inline"`
	Build    *StaticAppBuild `json:"build,omitempty"`
	Routing  string          `json:"routing"`
}

type StaticAppBuild struct {
	Command string `json:"command"`
	Dir     string `json:"dir"`
}

func LoadStaticAppData(path string, data []byte) (*StaticApp, error) {
	out := &StaticApp{}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load function config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	out.path = filepath.Dir(path)
	out.yamlPath = path
	out.yamlData = data
	out.typ = TypeStatic

	return out, nil
}

func (s *StaticApp) PluginType() *types.App {
	base := s.BasicApp.PluginType()

	if base.Properties == nil {
		base.Properties = make(map[string]interface{})
	}

	base.Properties["routing"] = s.Routing
	base.Properties["build"] = s.Build

	return base
}

func (s *StaticApp) Normalize(cfg *Project) error {
	if err := s.BasicApp.Normalize(cfg); err != nil {
		return err
	}

	s.Routing = strings.ToLower(s.Routing)

	if s.Routing == "" {
		s.Routing = StaticAppRoutingReact
	}

	if s.Build == nil {
		s.Build = &StaticAppBuild{}
	}

	if s.Build.Dir == "" {
		s.Build.Dir = DefaultStaticAppBuildDir
	}

	err := func() error {
		found := false

		for _, r := range StaticAppRoutings {
			if r == s.Routing {
				found = true

				break
			}
		}

		if !found {
			return s.yamlError("$.routing", fmt.Sprintf("%s has unknown routing value, did you mean \"routing: react\"?", s.typ))
		}

		return nil
	}()

	if err != nil {
		return fmt.Errorf("%s config validation failed.\nfile: %s\n%s", s.typ, s.yamlPath, err)
	}

	return nil
}

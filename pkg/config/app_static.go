package config

import (
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"github.com/outblocks/outblocks-plugin-go/types"
)

const (
	AppTypeStatic = "static"

	StaticAppRoutingReact    = "react"
	StaticAppRoutingGatsby   = "gatsby"
	StaticAppRoutingDisabled = "disabled"

	DefaultStaticAppBuildDir = "build"
)

var (
	StaticAppRoutings = []string{StaticAppRoutingReact, StaticAppRoutingGatsby, StaticAppRoutingDisabled}
)

type StaticApp struct {
	BasicApp `json:",inline"`
	Build    *StaticAppBuild `json:"build,omitempty"`
	Routing  string          `json:"routing"`
}

type StaticAppBuild struct {
	Command string `json:"command,omitempty"`
	Dir     string `json:"dir,omitempty"`
}

func LoadStaticAppData(path string, data []byte) (*StaticApp, error) {
	out := &StaticApp{
		BasicApp: BasicApp{
			AppRun:    &AppRun{},
			AppDeploy: &AppDeploy{},
		},
		Routing: StaticAppRoutingReact,
		Build: &StaticAppBuild{
			Dir: DefaultStaticAppBuildDir,
		},
	}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load function config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	out.yamlPath = path
	out.yamlData = data

	return out, nil
}

func (s *StaticApp) Validate() error {
	return validation.ValidateStruct(s,
		validation.Field(&s.Routing, validation.In(util.InterfaceSlice(StaticAppRoutings)...)),
		validation.Field(&s.AppURL, validation.Required),
	)
}

func (s *StaticApp) PluginType() *types.App {
	base := s.BasicApp.PluginType()

	if base.Properties == nil {
		base.Properties = make(map[string]interface{})
	}

	base.Properties["routing"] = s.Routing
	base.Properties["build"] = s.Build
	base.Properties["run"] = s.AppRun

	return base
}

func (s *StaticApp) Normalize(cfg *Project) error {
	if err := s.BasicApp.Normalize(cfg); err != nil {
		return err
	}

	s.Routing = strings.ToLower(s.Routing)

	return nil
}

func (s *StaticApp) SupportsLocal() bool {
	return true
}

package config

import (
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
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
	BasicApp                  `json:",inline"`
	types.StaticAppProperties `json:",inline"`
}

func LoadStaticAppData(path string, data []byte) (*StaticApp, error) {
	out := &StaticApp{
		BasicApp: BasicApp{
			AppRun:    &AppRun{},
			AppDeploy: &AppDeploy{},
		},
		StaticAppProperties: types.StaticAppProperties{
			Build: &types.StaticAppBuild{
				Dir: DefaultStaticAppBuildDir,
			},
			Routing: StaticAppRoutingReact,
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

	props, err := s.StaticAppProperties.Encode()
	if err != nil {
		panic(err)
	}

	base.Properties = plugin_util.MergeMaps(base.Properties, props)

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

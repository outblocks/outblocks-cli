package config

import (
	"strings"

	"github.com/23doors/go-yaml"
	"github.com/23doors/go-yaml/ast"
	"github.com/ansel1/merry/v2"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/internal/validator"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

const (
	AppTypeStatic = "static"

	StaticAppRoutingReact    = "react"
	StaticAppRoutingGatsby   = "gatsby"
	StaticAppRoutingDisabled = "disabled"

	DefaultStaticAppBuildDir       = "build"
	DefaultStaticAppBasicAuthRealm = "restricted"
)

var (
	StaticAppRoutings = []string{StaticAppRoutingReact, StaticAppRoutingGatsby, StaticAppRoutingDisabled}
)

type StaticApp struct {
	BasicApp                  `json:",inline"`
	types.StaticAppProperties `json:",inline"`
}

func LoadStaticAppData(path string, n ast.Node) (*StaticApp, error) {
	out := &StaticApp{
		BasicApp: *NewBasicApp(),
		StaticAppProperties: types.StaticAppProperties{
			Build: &types.StaticAppBuild{
				Dir: DefaultStaticAppBuildDir,
			},
			BasicAuth: &types.StaticAppBasicAuth{
				Realm: DefaultStaticAppBasicAuthRealm,
			},
			Routing: StaticAppRoutingReact,
		},
	}

	if err := yaml.NodeToValue(n, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, merry.Errorf("load function config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	out.yamlPath = path
	out.yamlData = []byte(n.String())

	return out, nil
}

func (s *StaticApp) Validate() error {
	return validation.ValidateStruct(s,
		validation.Field(&s.Routing, validation.In(util.InterfaceSlice(StaticAppRoutings)...)),
		validation.Field(&s.AppURL, validation.Required),
	)
}

func (s *StaticApp) Proto() *apiv1.App {
	base := s.BasicApp.Proto()

	props, err := s.StaticAppProperties.Encode()
	if err != nil {
		panic(err)
	}

	mergedProps := plugin_util.MergeMaps(base.Properties.AsMap(), props)
	base.Properties = plugin_util.MustNewStruct(mergedProps)

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

func (s *StaticApp) BuildProto() *apiv1.AppBuild {
	return nil
}

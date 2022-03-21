package config

import (
	"fmt"

	"github.com/ansel1/merry/v2"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/outblocks/outblocks-cli/internal/validator"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

const (
	AppTypeService = "service"
)

type ServiceApp struct {
	BasicApp                   `json:",inline"`
	types.ServiceAppProperties `json:",inline"`

	AppBuild *apiv1.AppBuild `json:"-"`
}

func LoadServiceAppData(path string, n ast.Node) (App, error) {
	out := &ServiceApp{
		BasicApp: *NewBasicApp(),
		ServiceAppProperties: types.ServiceAppProperties{
			Build: &types.ServiceAppBuild{
				Dockerfile: "Dockerfile",
			},
		},
		AppBuild: &apiv1.AppBuild{},
	}

	if err := yaml.NodeToValue(n, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, merry.Errorf("load service config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	out.AppBuild.LocalDockerImage = fmt.Sprintf("outblocks/%s", out.ID())

	out.yamlPath = path
	out.yamlData = []byte(n.String())

	return out, nil
}

func (s *ServiceApp) Validate() error {
	return validation.ValidateStruct(s,
		validation.Field(&s.AppURL, validation.When(!s.Private, validation.Required)),
	)
}

func (s *ServiceApp) SupportsLocal() bool {
	return true
}

func (s *ServiceApp) Proto() *apiv1.App {
	base := s.BasicApp.Proto()

	props, err := s.ServiceAppProperties.Encode()
	if err != nil {
		panic(err)
	}

	mergedProps := plugin_util.MergeMaps(base.Properties.AsMap(), props)
	base.Properties = plugin_util.MustNewStruct(mergedProps)

	return base
}

func (s *ServiceApp) BuildProto() *apiv1.AppBuild {
	return s.AppBuild
}

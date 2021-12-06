package config

import (
	"fmt"

	"github.com/goccy/go-yaml"
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
}

func LoadServiceAppData(path string, data []byte) (App, error) {
	out := &ServiceApp{
		BasicApp: *NewBasicApp(),
		ServiceAppProperties: types.ServiceAppProperties{
			Build: &types.ServiceAppBuild{
				Dockerfile: "Dockerfile",
			},
		},
	}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load service config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	out.LocalDockerImage = fmt.Sprintf("outblocks/%s", out.ID())

	out.yamlPath = path
	out.yamlData = data

	return out, nil
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

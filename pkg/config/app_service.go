package config

import (
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/validator"
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
		BasicApp: BasicApp{
			AppRun:    &AppRun{},
			AppDeploy: &AppDeploy{},
		},
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
	return false
}

func (s *ServiceApp) Validate() error {
	return validation.ValidateStruct(s,
		validation.Field(&s.AppURL, validation.Required),
	)
}

func (s *ServiceApp) PluginType() *types.App {
	base := s.BasicApp.PluginType()

	props, err := s.ServiceAppProperties.Encode()
	if err != nil {
		panic(err)
	}

	base.Properties = plugin_util.MergeMaps(base.Properties, props)

	return base
}

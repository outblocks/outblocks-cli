package config

import (
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"github.com/outblocks/outblocks-plugin-go/types"
)

const (
	AppTypeService = "service"
)

type ServiceApp struct {
	BasicApp   `json:",inline"`
	Build      *ServiceAppBuild `json:"build,omitempty"`
	DockerHash string           `json:"-"`
}

type ServiceAppBuild struct {
	Dockerfile    string `json:"dockerfile,omitempty"`
	DockerContext string `json:"context,omitempty"`
}

func LoadServiceAppData(path string, data []byte) (App, error) {
	out := &ServiceApp{
		BasicApp: BasicApp{
			AppRun:    &AppRun{},
			AppDeploy: &AppDeploy{},
		},
		Build: &ServiceAppBuild{
			Dockerfile: "Dockerfile",
		},
	}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load service config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	out.yamlPath = path
	out.yamlData = data

	return out, nil
}

func (s *ServiceApp) SupportsLocal() bool {
	return false
}

func (s *ServiceApp) LocalDockerImage() string {
	return fmt.Sprintf("outblocks/%s", s.ID())
}

func (s *ServiceApp) Validate() error {
	return validation.ValidateStruct(s,
		validation.Field(&s.AppURL, validation.Required),
	)
}

func (s *ServiceApp) PluginType() *types.App {
	base := s.BasicApp.PluginType()

	if base.Properties == nil {
		base.Properties = make(map[string]interface{})
	}

	base.Properties["build"] = s.Build
	base.Properties["local_docker_image"] = s.LocalDockerImage()
	base.Properties["local_docker_hash"] = s.DockerHash

	return base
}

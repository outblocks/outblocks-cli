package config

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/validator"
)

const (
	AppTypeService = "service"
)

type ServiceApp struct {
	BasicApp `json:",inline"`
}

func LoadServiceAppData(path string, data []byte) (App, error) {
	out := &ServiceApp{
		BasicApp: BasicApp{
			AppRun:    &AppRun{},
			AppDeploy: &AppDeploy{},
		},
	}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load function config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	out.yamlPath = path
	out.yamlData = data
	out.typ = AppTypeService

	return out, nil
}

func (s *ServiceApp) SupportsLocal() bool {
	return false
}

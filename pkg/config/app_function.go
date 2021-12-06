package config

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/validator"
)

const (
	AppTypeFunction = "function"
)

type FunctionApp struct {
	BasicApp `json:",inline"`
}

func LoadFunctionAppData(path string, data []byte) (App, error) {
	out := &FunctionApp{
		BasicApp: BasicApp{
			AppRun:    &AppRunInfo{},
			AppDeploy: &AppDeployInfo{},
		},
	}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load function config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	out.yamlPath = path
	out.yamlData = data

	return out, nil
}

func (s *FunctionApp) SupportsLocal() bool {
	return false
}

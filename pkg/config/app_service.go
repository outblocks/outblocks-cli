package config

import (
	"fmt"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/validator"
)

const (
	TypeService = "service"
)

type ServiceApp struct {
	BasicApp `json:",inline"`
}

func LoadServiceAppData(path string, data []byte) (App, error) {
	out := &ServiceApp{}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load function config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	out.Path = filepath.Dir(path)
	out.yamlPath = path
	out.yamlData = data
	out.typ = TypeService

	return out, nil
}

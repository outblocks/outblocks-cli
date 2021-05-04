package config

import (
	"fmt"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"github.com/pterm/pterm"
)

const (
	TypeService = "service"
)

type ServiceConfig struct {
	BasicApp `json:",inline"`
}

func LoadServiceConfigData(path string, data []byte) (App, error) {
	out := &ServiceConfig{}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load function config %s error: \n%s", path, yaml.FormatError(err, pterm.PrintColor, true))
	}

	out.Path = filepath.Dir(path)
	out.yamlPath = path
	out.data = data
	out.typ = TypeService

	return out, nil
}

package config

import (
	"fmt"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/validator"
)

const (
	TypeFunction = "function"
)

type FunctionApp struct {
	BasicApp `json:",inline"`
}

func LoadFunctionAppData(path string, data []byte) (App, error) {
	out := &FunctionApp{}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load function config %s error: \n%s", path, yaml.FormatErrorDefault(err))
	}

	out.path = filepath.Dir(path)
	out.yamlPath = path
	out.yamlData = data
	out.typ = TypeFunction

	return out, nil
}

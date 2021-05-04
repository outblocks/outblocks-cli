package config

import (
	"fmt"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"github.com/pterm/pterm"
)

const (
	TypeFunction = "function"
)

type FunctionConfig struct {
	BasicApp `json:",inline"`
}

func LoadFunctionConfigData(path string, data []byte) (App, error) {
	out := &FunctionConfig{}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load function config %s error: \n%s", path, yaml.FormatError(err, pterm.PrintColor, true))
	}

	out.Path = filepath.Dir(path)
	out.yamlPath = path
	out.data = data
	out.typ = TypeFunction

	return out, nil
}

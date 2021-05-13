package config

import (
	"fmt"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"github.com/outblocks/outblocks-plugin-go/types"
	"github.com/pterm/pterm"
)

const (
	TypeStatic = "static"
)

type StaticApp struct {
	BasicApp `json:",inline"`
	Error404 string `json:"error404"`
}

func LoadStaticAppData(path string, data []byte) (*StaticApp, error) {
	out := &StaticApp{}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, fmt.Errorf("load function config %s error: \n%s", path, yaml.FormatError(err, pterm.PrintColor, true))
	}

	out.Path = filepath.Dir(path)
	out.yamlPath = path
	out.yamlData = data
	out.typ = TypeStatic

	return out, nil
}

func (s *StaticApp) PluginType() *types.App {
	base := s.BasicApp.PluginType()

	if base.Properties == nil {
		base.Properties = make(map[string]interface{})
	}

	base.Properties["error404"] = s.Error404

	return base
}

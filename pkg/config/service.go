package config

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/pterm/pterm"
)

type ServiceConfig struct {
	Name   string                 `json:"name"`
	URL    string                 `json:"url"`
	Deploy string                 `json:"deploy"`
	Needs  map[string]*Need       `json:"needs"`
	Other  map[string]interface{} `yaml:"-,remain"`

	Path string `json:"-"`
	data []byte
}

type Need struct {
	Name   string                 `json:"name"`
	Type   string                 `json:"type"`
	Deploy string                 `json:"deploy"`
	Other  map[string]interface{} `yaml:"-,remain"`
}

func LoadServiceConfigData(path string, data []byte) (*ServiceConfig, error) {
	out := &ServiceConfig{
		Path: path,
		data: data,
	}

	if err := yaml.Unmarshal(data, out); err != nil {
		return nil, fmt.Errorf("load function config %s error: \n%s", path, yaml.FormatError(err, pterm.PrintColor, true))
	}

	return out, nil
}

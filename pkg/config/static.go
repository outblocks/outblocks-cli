package config

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/pterm/pterm"
)

type StaticConfig struct {
	Name   string                 `json:"name"`
	URL    string                 `json:"url"`
	Deploy string                 `json:"deploy"`
	Needs  map[string]*Need       `json:"needs"`
	Other  map[string]interface{} `yaml:"-,remain"`

	Path string `json:"-"`
	data []byte
}

func LoadStaticConfigData(path string, data []byte) (*StaticConfig, error) {
	out := &StaticConfig{
		Path: path,
		data: data,
	}

	if err := yaml.Unmarshal(data, out); err != nil {
		return nil, fmt.Errorf("load function config %s error: \n%s", path, yaml.FormatError(err, pterm.PrintColor, true))
	}

	return out, nil
}

package config

import "github.com/outblocks/outblocks-cli/pkg/plugins"

const (
	StateLocal = "local"
)

type ProjectState struct {
	Type  string                 `json:"type"`
	Other map[string]interface{} `yaml:"-,remain"`

	plugin *plugins.Plugin
}

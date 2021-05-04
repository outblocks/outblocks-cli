package config

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type ProjectDependency struct {
	Type   string                 `json:"type"`
	Deploy string                 `json:"deploy"`
	Other  map[string]interface{} `yaml:"-,remain"`

	deployPlugin *plugins.Plugin
	runPlugin    *plugins.Plugin
}

func (d *ProjectDependency) Validate() error {
	return validation.ValidateStruct(d,
		validation.Field(&d.Type, validation.Required),
	)
}

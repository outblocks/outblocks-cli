package config

import (
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-plugin-go/types"
)

type Dependency struct {
	Name   string                 `json:"-"`
	Type   string                 `json:"type"`
	Deploy string                 `json:"deploy"`
	Other  map[string]interface{} `yaml:"-,remain"`

	deployPlugin *plugins.Plugin
	runPlugin    *plugins.Plugin
}

func (d *Dependency) Validate() error {
	return validation.ValidateStruct(d,
		validation.Field(&d.Type, validation.Required),
	)
}

func (d *Dependency) PluginType() *types.Dependency {
	return &types.Dependency{
		ID:         d.ID(),
		Name:       d.Name,
		Type:       d.Type,
		Deploy:     d.Deploy,
		Properties: d.Other,
	}
}

func (d *Dependency) Normalize(key string, cfg *Project) error {
	d.Type = strings.ToLower(d.Type)
	d.Deploy = strings.ToLower(d.Deploy)

	if d.Type == "" {
		return cfg.yamlError(fmt.Sprintf("$.dependencies.%s.type", key), "dependency.type cannot be empty")
	}

	return nil
}

func (d *Dependency) Check(key string, cfg *Project) error {
	// Check deploy plugin.
	for _, plug := range cfg.plugins {
		if !plug.HasAction(plugins.ActionDeploy) {
			continue
		}

		if !plug.SupportsType(d.Type, d.Deploy, d.Other) {
			continue
		}

		d.deployPlugin = plug
		d.Deploy = plug.Name

		break
	}

	if d.deployPlugin == nil {
		return cfg.yamlError(fmt.Sprintf("$.dependencies.%s", key), "dependency has no matching deployment plugin available")
	}

	// Check run plugin.
	for _, plug := range cfg.plugins {
		if !plug.HasAction(plugins.ActionRun) {
			continue
		}

		if !plug.SupportsType(d.Type, d.Deploy, d.Other) {
			continue
		}

		d.runPlugin = plug
	}

	if d.runPlugin == nil {
		return cfg.yamlError(fmt.Sprintf("$.dependencies.%s", key), "dependency has no matching run plugin available")
	}

	return nil
}

func (d *Dependency) DeployPlugin() *plugins.Plugin {
	return d.deployPlugin
}

func (d *Dependency) RunPlugin() *plugins.Plugin {
	return d.runPlugin
}

func (d *Dependency) ID() string {
	return d.Name
}

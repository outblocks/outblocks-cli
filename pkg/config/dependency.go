package config

import (
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

type Dependency struct {
	Name   string                 `json:"-"`
	Type   string                 `json:"type"`
	Deploy *DependencyDeploy      `json:"deploy,omitempty"`
	Run    *DependencyRun         `json:"run,omitempty"`
	Other  map[string]interface{} `yaml:"-,remain"`

	cfg          *Project
	deployPlugin *plugins.Plugin
	runPlugin    *plugins.Plugin
}

type DependencyRun struct {
	Plugin string                 `json:"plugin,omitempty"`
	Port   int                    `json:"port,omitempty"`
	Other  map[string]interface{} `yaml:"-,remain"`
}

type DependencyDeploy struct {
	Plugin string                 `json:"plugin,omitempty"`
	Other  map[string]interface{} `yaml:"-,remain"`
}

func (d *Dependency) Validate() error {
	return validation.ValidateStruct(d,
		validation.Field(&d.Type, validation.Required),
	)
}

func (d *Dependency) Proto() *apiv1.Dependency {
	var deployPluginName string

	if d.DeployPlugin() != nil {
		deployPluginName = d.DeployPlugin().Name
	}

	return &apiv1.Dependency{
		Id:           d.ID(),
		DeployPlugin: deployPluginName,
		Name:         d.Name,
		Type:         d.Type,
		Properties:   plugin_util.MustNewStruct(d.Other),
	}
}

func (d *Dependency) Normalize(key string, cfg *Project) error {
	d.Type = strings.ToLower(d.Type)

	if d.Deploy == nil {
		d.Deploy = &DependencyDeploy{}
	}

	if d.Run == nil {
		d.Run = &DependencyRun{}
	}

	d.Deploy.Plugin = strings.ToLower(d.Deploy.Plugin)
	d.Run.Plugin = strings.ToLower(d.Run.Plugin)

	if d.Type == "" {
		return cfg.yamlError(fmt.Sprintf("$.dependencies.%s.type", key), "dependency.type cannot be empty")
	}

	return nil
}

func (d *Dependency) Check(key string, cfg *Project) error {
	// Check deploy plugin.
	deployPlugin := d.Deploy.Plugin

	for _, plug := range cfg.loadedPlugins {
		if !plug.HasAction(plugins.ActionDeploy) {
			continue
		}

		if (deployPlugin != "" && deployPlugin != plug.Name) || !plug.SupportsType(d.Type, deployPlugin, d.Other) {
			continue
		}

		d.deployPlugin = plug
		d.Deploy.Plugin = plug.Name

		break
	}

	if d.deployPlugin == nil {
		return d.YAMLError("", "dependency has no matching deployment plugin available")
	}

	// Check run plugin.
	runPlugin := d.Run.Plugin

	for _, plug := range cfg.loadedPlugins {
		if !plug.HasAction(plugins.ActionRun) {
			continue
		}

		if (runPlugin != "" && runPlugin != plug.Name) || !plug.SupportsType(d.Type, d.Deploy.Plugin, d.Other) {
			continue
		}

		d.runPlugin = plug
		d.Run.Plugin = plug.Name
	}

	if d.runPlugin == nil {
		return d.YAMLError("", "dependency has no matching run plugin available")
	}

	return nil
}

func (d *Dependency) DeployPlugin() *plugins.Plugin {
	return d.deployPlugin
}

func (d *Dependency) RunPlugin() *plugins.Plugin {
	return d.runPlugin
}

func (d *Dependency) YAMLError(subPath, err string) error {
	return d.cfg.yamlError(fmt.Sprintf("$.dependencies.%s%s", d.Name, subPath), err)
}

func ComputeDependencyID(name string) string {
	return fmt.Sprintf("dep_%s", name)
}

func (d *Dependency) ID() string {
	return ComputeDependencyID(d.Name)
}

func (d *Dependency) SupportsLocal() bool {
	return false
}

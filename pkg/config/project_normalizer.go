package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type ProjectConfigNormalizer struct {
	*ProjectConfig
}

func NewProjectConfigNormalizer(cfg *ProjectConfig) *ProjectConfigNormalizer {
	return &ProjectConfigNormalizer{cfg}
}

// Initial first pass validation.
func (p *ProjectConfigNormalizer) Normalize() error {
	err := func() error {
		for i, plugin := range p.Plugins {
			if err := p.normalizePlugin(i, plugin); err != nil {
				return err
			}
		}

		for _, f := range p.functions {
			if err := NewFunctionConfigNormalizer(f, p.plugins).Normalize(); err != nil {
				return err
			}
		}

		for _, svc := range p.services {
			if err := NewServiceConfigNormalizer(svc, p.plugins).Normalize(); err != nil {
				return err
			}
		}

		for _, s := range p.static {
			if err := NewStaticConfigNormalizer(s, p.plugins).Normalize(); err != nil {
				return err
			}
		}

		return nil
	}()

	if err != nil {
		return fmt.Errorf("project config validation failed.\nfile: %s\n%s", p.Path, err)
	}

	return nil
}

// Logic validation after everything is loaded, e.g. check for supported types.
func (p *ProjectConfigNormalizer) FullCheck() error {
	// TODO: logic validation after everything is loaded, e.g. check for supported types
	err := func() error {
		for key, dep := range p.Dependencies {
			if err := p.checkDependency(key, dep); err != nil {
				return err
			}
		}

		return nil
	}()

	if err != nil {
		return fmt.Errorf("project config validation failed.\nfile: %s\n%s", p.Path, err)
	}

	return nil
}

func (p *ProjectConfigNormalizer) yamlError(path, s string) error {
	return fileutil.YAMLError(path, s, p.data)
}

func (p *ProjectConfigNormalizer) normalizePlugin(i int, plugin *ProjectPlugin) error {
	var err error

	if plugin.Version != "" {
		plugin.verRange, err = semver.ParseRange(plugin.Version)
		if err != nil {
			return p.yamlError(fmt.Sprintf("$.plugins[%d].version", i), "Plugin.Version is in invalid format")
		}
	}

	if plugin.Source != "" {
		u, err := url.Parse(plugin.Source)
		if err != nil {
			return p.yamlError(fmt.Sprintf("$.plugins[%d].source", i), "Plugin.Source is not a valid URL")
		}

		plugin.Source = u.String()
	}

	plugin.order = uint(i)

	return nil
}

func (p *ProjectConfigNormalizer) checkDependency(key string, dep *ProjectDependency) error {
	var (
		deploySupported bool
		runSupported    bool
	)

	dep.Type = strings.ToLower(dep.Type)
	dep.Deploy = strings.ToLower(dep.Deploy)

	if dep.Type == "" {
		return p.yamlError(fmt.Sprintf("$.dependencies.%s.type", key), "Dependency.Type cannot be empty")
	}

	// 	if dep.Deploy == "" {
	// 		return p.yamlError(fmt.Sprintf("$.dependencies[%s].deploy", key), "Dependency.Deploy cannot be empty")
	// }

	for _, plug := range p.plugins {
		if !plug.SupportsType(dep.Type, dep.Deploy, dep.Other) {
			continue
		}

		deploySupported = deploySupported || plug.HasAction(plugins.ActionDeploy)
		runSupported = runSupported || plug.HasAction(plugins.ActionRun)
	}

	// TODO: check if both are supported
	// return p.yamlError(fmt.Sprintf("$.dependencies.%s..type", key), "Dependency.Type is not supported by any plugins")

	// if dep.Type
	// if plugin.Version != "" {
	// 	plugin.verRange, err = semver.ParseRange(plugin.Version)
	// 	if err != nil {
	// 		return p.yamlError(fmt.Sprintf("$.plugins[%d].version", i), "Plugin.Version is in invalid format")
	// 	}
	// }

	// if plugin.Source != "" {
	// 	u, err := url.Parse(plugin.Source)
	// 	if err != nil {
	// 		return p.yamlError(fmt.Sprintf("$.plugins[%d].source", i), "Plugin.Source is not a valid URL")
	// 	}

	// 	plugin.Source = u.String()
	// }

	// plugin.order = uint(i)

	return nil
}

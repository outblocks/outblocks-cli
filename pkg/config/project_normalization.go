package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

// Initial first pass validation.
func (p *ProjectConfig) Normalize() error {
	err := func() error {
		for i, plugin := range p.Plugins {
			if err := p.normalizePlugin(i, plugin); err != nil {
				return err
			}
		}

		// Default to local statefile.
		if p.State == nil {
			p.State = &ProjectState{
				Type: StateLocal,
			}
		} else if p.State.Type == "" {
			p.State.Type = StateLocal
		}

		return nil
	}()

	if err != nil {
		return fmt.Errorf("project config validation failed.\nfile: %s\n\n%s", p.yamlPath, err)
	}

	err = func() error {
		for _, app := range p.apps {
			if err := app.Normalize(p); err != nil {
				return err
			}
		}

		return nil
	}()

	return err
}

// Logic validation after everything is loaded, e.g. check for supported types.
func (p *ProjectConfig) FullCheck() error {
	err := func() error {
		for key, dep := range p.Dependencies {
			if err := p.checkDependency(key, dep); err != nil {
				return err
			}
		}

		if err := p.checkState(p.State); err != nil {
			return err
		}

		return nil
	}()

	if err != nil {
		return fmt.Errorf("project config check failed.\nfile: %s\n\n%s", p.yamlPath, err)
	}

	err = func() error {
		for _, app := range p.apps {
			if err := app.Check(p); err != nil {
				return err
			}
		}

		return nil
	}()

	return err
}

func (p *ProjectConfig) yamlError(path, msg string) error {
	return fileutil.YAMLError(path, msg, p.data)
}

func (p *ProjectConfig) normalizePlugin(i int, plugin *ProjectPlugin) error {
	var err error

	if plugin.Version != "" {
		plugin.verRange, err = semver.ParseRange(plugin.Version)
		if err != nil {
			return p.yamlError(fmt.Sprintf("$.plugins[%d].version", i), "plugin.version is in invalid format")
		}
	}

	if plugin.Source != "" {
		u, err := url.Parse(plugin.Source)
		if err != nil {
			return p.yamlError(fmt.Sprintf("$.plugins[%d].source", i), "plugin.source is not a valid URL")
		}

		plugin.Source = u.String()
	}

	plugin.order = uint(i)

	return nil
}

func (p *ProjectConfig) checkDependency(key string, dep *ProjectDependency) error {
	dep.Type = strings.ToLower(dep.Type)
	dep.Deploy = strings.ToLower(dep.Deploy)

	if dep.Type == "" {
		return p.yamlError(fmt.Sprintf("$.dependencies.%s.type", key), "dependency.type cannot be empty")
	}

	// Check deploy plugin.
	for _, plug := range p.plugins {
		if !plug.HasAction(plugins.ActionDeploy) {
			continue
		}

		if !plug.SupportsType(dep.Type, dep.Deploy, dep.Other) {
			continue
		}

		dep.deployPlugin = plug
		dep.Deploy = plug.Name

		break
	}

	if dep.deployPlugin == nil {
		return p.yamlError(fmt.Sprintf("$.dependencies.%s", key), "dependency has no deployment plugin available")
	}

	// Check run plugin.
	for _, plug := range p.plugins {
		if !plug.HasAction(plugins.ActionRun) {
			continue
		}

		if !plug.SupportsType(dep.Type, dep.Deploy, dep.Other) {
			continue
		}

		dep.runPlugin = plug
	}

	if dep.runPlugin == nil {
		return p.yamlError(fmt.Sprintf("$.dependencies.%s", key), "dependency has no run plugin available")
	}

	return nil
}

func (p *ProjectConfig) checkState(state *ProjectState) error {
	state.Type = strings.ToLower(state.Type)

	if state.Type == StateLocal {
		return nil
	}

	// Check plugin.
	for _, plug := range p.plugins {
		for _, typ := range plug.StateTypes {
			if typ == state.Type {
				state.plugin = plug
			}
		}
	}

	if state.plugin == nil {
		return p.yamlError("$.state", "state has no supported plugin available")
	}

	return nil
}

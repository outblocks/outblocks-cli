package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-plugin-go/types"
)

type App interface {
	Normalize(cfg *ProjectConfig) error
	Check(cfg *ProjectConfig) error
	Type() string
	PluginType() *types.App

	DeployPlugin() *plugins.Plugin
	RunPlugin() *plugins.Plugin
	DNSPlugin() *plugins.Plugin
}

type BasicApp struct {
	Name   string                 `json:"name"`
	URL    string                 `json:"url"`
	Deploy string                 `json:"deploy"`
	Needs  map[string]AppNeed     `json:"needs"`
	Other  map[string]interface{} `yaml:"-,remain"`

	Path         string `json:"-"`
	yamlPath     string
	yamlData     []byte
	deployPlugin *plugins.Plugin
	dnsPlugin    *plugins.Plugin
	runPlugin    *plugins.Plugin
	typ          string
}

func (a *BasicApp) Normalize(cfg *ProjectConfig) error {
	if a.Name == "" {
		a.Name = filepath.Base(a.Path)
	}

	err := func() error {
		for name, n := range a.Needs {
			if err := n.Normalize(name, cfg, a.yamlData); err != nil {
				return err
			}
		}

		return nil
	}()

	if err != nil {
		return fmt.Errorf("%s config validation failed.\nfile: %s\n%s", a.typ, a.yamlPath, err)
	}

	return nil
}

func (a *BasicApp) Check(cfg *ProjectConfig) error {
	a.Deploy = strings.ToLower(a.Deploy)

	// Check deploy plugin.
	for _, plug := range cfg.plugins {
		if !plug.HasAction(plugins.ActionDeploy) {
			continue
		}

		if (a.Deploy != "" && a.Deploy != plug.Name) || !plug.SupportsApp(a.typ) {
			continue
		}

		a.deployPlugin = plug
		a.Deploy = plug.Name

		break
	}

	if a.deployPlugin == nil {
		return fmt.Errorf("%s has no matching deployment plugin available.\nfile: %s", a.typ, a.yamlPath)
	}

	// Check run plugin.
	for _, plug := range cfg.plugins {
		if !plug.HasAction(plugins.ActionRun) {
			continue
		}

		if !plug.SupportsApp(a.typ) {
			continue
		}

		a.runPlugin = plug
	}

	if a.runPlugin == nil {
		return fmt.Errorf("%s has no matching run plugin available.\nfile: %s", a.typ, a.yamlPath)
	}

	// Check dns plugin.
	if a.URL != "" {
		a.dnsPlugin = cfg.FindDNSPlugin(a.URL)

		if a.dnsPlugin == nil {
			return a.yamlError("$.url", fmt.Sprintf("%s has no matching dns plugin available.", a.typ))
		}
	}

	return nil
}

func (a *BasicApp) yamlError(path, msg string) error {
	return fmt.Errorf("file: %s\n%s", a.yamlPath, fileutil.YAMLError(path, msg, a.yamlData))
}

func (a *BasicApp) Type() string {
	return a.typ
}

func (a *BasicApp) PluginType() *types.App {
	needs := make(map[string]*types.AppNeed, len(a.Needs))

	for k, n := range a.Needs {
		needs[k] = n.PluginType()
	}

	return &types.App{
		Name:       a.Name,
		Path:       a.Path,
		Type:       a.Type(),
		URL:        a.URL,
		Deploy:     a.Deploy,
		Needs:      needs,
		Properties: a.Other,
	}
}

func (a *BasicApp) DeployPlugin() *plugins.Plugin {
	return a.deployPlugin
}

func (a *BasicApp) DNSPlugin() *plugins.Plugin {
	return a.dnsPlugin
}

func (a *BasicApp) RunPlugin() *plugins.Plugin {
	return a.runPlugin
}

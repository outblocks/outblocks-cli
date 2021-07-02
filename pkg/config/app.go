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
	ID() string
	Name() string
	URL() string
	Normalize(cfg *Project) error
	Check(cfg *Project) error
	Type() string
	Path() string
	PluginType() *types.App

	DeployPlugin() *plugins.Plugin
	RunPlugin() *plugins.Plugin
	DNSPlugin() *plugins.Plugin
}

type BasicApp struct {
	AppName string                 `json:"name"`
	AppURL  string                 `json:"url"`
	Deploy  string                 `json:"deploy"`
	Needs   map[string]*AppNeed    `json:"needs"`
	Other   map[string]interface{} `yaml:"-,remain"`

	path         string
	yamlPath     string
	yamlData     []byte
	deployPlugin *plugins.Plugin
	dnsPlugin    *plugins.Plugin
	runPlugin    *plugins.Plugin
	typ          string
}

func (a *BasicApp) Normalize(cfg *Project) error {
	if a.AppName == "" {
		a.AppName = filepath.Base(a.path)
	}

	if a.AppURL != "" {
		a.AppURL = strings.ToLower(a.AppURL)

		if strings.Count(a.AppURL, "/") == 0 {
			a.AppURL += "/"
		}
	}

	err := func() error {
		for name, n := range a.Needs {
			if n == nil {
				a.Needs[name] = &AppNeed{}
			}
		}

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

func (a *BasicApp) Check(cfg *Project) error {
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
	if a.AppURL != "" {
		a.dnsPlugin = cfg.FindDNSPlugin(a.AppURL)
	}

	for k, need := range a.Needs {
		if need.dep.deployPlugin != a.deployPlugin {
			return a.yamlError(fmt.Sprintf("$.needs[%s]", k), fmt.Sprintf("%s needs a dependency that uses different deployment plugin.", a.typ))
		}

		if need.dep.runPlugin != a.runPlugin {
			return a.yamlError(fmt.Sprintf("$.needs[%s]", k), fmt.Sprintf("%s needs a dependency that uses different run plugin.", a.typ))
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

func (a *BasicApp) Path() string {
	return a.path
}

func (a *BasicApp) PluginType() *types.App {
	needs := make(map[string]*types.AppNeed, len(a.Needs))

	for k, n := range a.Needs {
		needs[k] = n.PluginType()
	}

	return &types.App{
		ID:         a.ID(),
		Name:       a.AppName,
		Type:       a.Type(),
		URL:        a.AppURL,
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

func (a *BasicApp) Name() string {
	return a.AppName
}

func (a *BasicApp) URL() string {
	return a.AppURL
}

func (a *BasicApp) ID() string {
	return fmt.Sprintf("app_%s_%s", a.typ, a.AppName)
}

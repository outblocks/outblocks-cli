package config

import (
	"fmt"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type App interface {
	Normalize(cfg *ProjectConfig) error
	Check(cfg *ProjectConfig) error
}

type BasicApp struct {
	Name   string                 `json:"name"`
	URL    string                 `json:"url"`
	Deploy string                 `json:"deploy"`
	Needs  map[string]Need        `json:"needs"`
	Other  map[string]interface{} `yaml:"-,remain"`

	Path         string `json:"-"`
	yamlPath     string
	data         []byte
	deployPlugin *plugins.Plugin
	runPlugin    *plugins.Plugin
	typ          string
}

func (a *BasicApp) Normalize(cfg *ProjectConfig) error {
	err := func() error {
		for name, n := range a.Needs {
			if err := n.Normalize(name, cfg, a.data); err != nil {
				return err
			}
		}

		return nil
	}()

	if err != nil {
		return fmt.Errorf("%s config validation failed.\nfile: %s\n\n%s", a.typ, a.yamlPath, err)
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
		return fmt.Errorf("%s has no deployment plugin available.\nfile: %s", a.typ, a.yamlPath)
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
		return fmt.Errorf("%s has no run plugin available.\nfile: %s", a.typ, a.yamlPath)
	}

	return nil
}

func (a *BasicApp) yamlError(path, msg string) error {
	return fileutil.YAMLError(path, msg, a.data)
}

func (a *BasicApp) Type() string {
	return a.typ
}

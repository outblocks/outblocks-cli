package cmd

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/pkg/cli"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/pterm/pterm"
)

func (e *Executor) loadProjectConfig(ctx *cli.Context, vals map[string]interface{}) error {
	cfg, err := config.LoadProjectConfig(vals)
	if err != nil {
		return err
	}

	v := config.NewProjectConfigNormalizer(cfg)
	if err := v.Normalize(); err != nil {
		return err
	}

	e.loader = plugins.NewLoader(filepath.Dir(cfg.Path), e.v.GetString("plugins_cache_dir"))

	if err := e.loadPlugins(ctx, cfg); err != nil {
		return err
	}

	if err := cfg.LoadApps(); err != nil {
		return err
	}

	if err := v.FullCheck(); err != nil {
		return err
	}

	e.cfg = cfg

	return nil
}

func (e *Executor) loadPlugins(ctx *cli.Context, cfg *config.ProjectConfig) error {
	plugs := make([]*plugins.Plugin, len(cfg.Plugins))
	pluginsToDownload := make(map[int]*config.ProjectPlugin)

	for i, plug := range cfg.Plugins {
		plugin, err := e.loader.LoadPlugin(plug.Name, plug.Source, plug.VerRange(), cfg.PluginLock(plug))
		if err != nil {
			if err != plugins.ErrPluginNotFound {
				return err
			}

			pluginsToDownload[i] = plug

			continue
		}

		plugs[i] = plugin
	}

	if len(pluginsToDownload) != 0 {
		prog, _ := pterm.DefaultProgressbar.WithTotal(len(pluginsToDownload)).WithTitle("Downloading plugins").Start()

		for i, plug := range pluginsToDownload {
			prog.Title = fmt.Sprintf("Downloading '%s' plugin", plug.Name)
			prog.Add(0) // force title update

			plugin, err := e.loader.DownloadPlugin(ctx, plug.Name, plug.VerRange(), plug.Source, cfg.PluginLock(plug))
			plugs[i] = plugin

			if err != nil {
				_, _ = prog.Stop()

				return fmt.Errorf("unable to load '%s' plugin: %w", plug.Name, err)
			}

			prog.Increment()
			pterm.Success.Printf("Downloaded '%s'\n", plug.Name)
		}

		_, _ = prog.Stop()
	}

	return cfg.LoadPlugins(plugs)
}

func (e *Executor) saveLockfile() error {
	lock := e.cfg.Lockfile()

	data, err := yaml.Marshal(lock)
	if err != nil {
		return fmt.Errorf("marshaling lockfile error: %w", err)
	}

	if err := ioutil.WriteFile(filepath.Join(e.cfg.BaseDir, config.LockfileName), data, 0755); err != nil {
		return fmt.Errorf("writing lockfile error: %w", err)
	}

	return nil
}

package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/pterm/pterm"
)

func (e *Executor) loadProjectConfig(ctx context.Context, vals map[string]interface{}) error {
	cfg, err := config.LoadProjectConfig(vals)
	if err != nil {
		return err
	}

	if err := cfg.LoadApps(); err != nil {
		return err
	}

	if err := cfg.Normalize(); err != nil {
		return err
	}

	e.loader = plugins.NewLoader(cfg.Path, e.v.GetString("plugins_cache_dir"))

	if err := e.loadPlugins(ctx, cfg); err != nil {
		return err
	}

	if err := cfg.FullCheck(); err != nil {
		return err
	}

	e.cfg = cfg

	return nil
}

func (e *Executor) loadPlugins(ctx context.Context, cfg *config.Project) error {
	plugs := make([]*plugins.Plugin, len(cfg.Plugins))
	pluginsToDownload := make(map[int]*config.Plugin)

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
		prog, _ := e.log.ProgressBar().WithTotal(len(pluginsToDownload)).WithTitle("Downloading plugins").Start()

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

	// Normalize and start plugins.
	for _, plug := range plugs {
		if err := plug.Normalize(); err != nil {
			return err
		}
	}

	for i, plug := range plugs {
		plug := plug
		plugConfig := cfg.Plugins[i]
		prefix := fmt.Sprintf("$.plugins[%d]", i)

		if err := plug.Prepare(ctx, e.Log(), cfg.Name, cfg.Path, plugConfig.Other, prefix, cfg.YAMLData()); err != nil {
			return fmt.Errorf("error starting plugin '%s': %w", plug.Name, err)
		}
	}

	cfg.SetPlugins(plugs)

	return nil
}

func (e *Executor) cleanupProject() error {
	if e.cfg == nil {
		return nil
	}

	for _, plug := range e.cfg.LoadedPlugins() {
		if err := plug.Stop(); err != nil {
			return fmt.Errorf("error stopping plugin '%s': %w", plug.Name, err)
		}
	}

	return nil
}

func (e *Executor) saveLockfile() error {
	lock := e.cfg.Lockfile()

	data, err := yaml.MarshalWithOptions(lock, yaml.UseJSONMarshaler())
	if err != nil {
		return fmt.Errorf("marshaling lockfile error: %w", err)
	}

	if err := ioutil.WriteFile(filepath.Join(e.cfg.Path, config.LockfileName), data, 0755); err != nil {
		return fmt.Errorf("writing lockfile error: %w", err)
	}

	return nil
}

package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

func (e *Executor) loadProjectConfig(ctx context.Context, vals map[string]interface{}, skipLoadPlugins bool) error {
	cfg, err := config.LoadProjectConfig(vals, &config.ProjectOptions{
		Env: e.opts.env,
	})
	if err != nil {
		return err
	}

	if err := cfg.LoadApps(); err != nil {
		return err
	}

	if err := cfg.Normalize(); err != nil {
		return err
	}

	e.loader = plugins.NewLoader(cfg.Path, e.PluginsCacheDir())

	if skipLoadPlugins {
		if err := cfg.LoadPlugins(ctx, e.log, e.loader); err != nil {
			return err
		}
	}

	if err := cfg.FullCheck(); err != nil {
		return err
	}

	e.cfg = cfg

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

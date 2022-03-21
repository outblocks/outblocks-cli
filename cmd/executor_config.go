package cmd

import (
	"context"
	"path/filepath"

	"github.com/ansel1/merry/v2"
	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type LoadProjectOptions struct {
	Mode            config.LoadMode
	SkipLoadPlugins bool
	SkipCheck       bool
}

func (e *Executor) loadProject(ctx context.Context, cfgPath, hostAddr string, vals map[string]interface{}, loadProjectOpts LoadProjectOptions, loadAppsMode config.LoadMode) error {
	cfg, err := config.LoadProjectConfig(cfgPath, vals, loadProjectOpts.Mode, &config.ProjectOptions{
		Env: e.opts.env,
	})
	if err != nil {
		return err
	}

	if err := cfg.LoadApps(loadAppsMode); err != nil {
		return err
	}

	if err := cfg.Normalize(); err != nil {
		return err
	}

	e.loader = plugins.NewLoader(cfg.Dir, e.PluginsCacheDir())
	e.cfg = cfg

	if !loadProjectOpts.SkipLoadPlugins {
		if err := cfg.LoadPlugins(ctx, e.log, e.loader, hostAddr); err != nil {
			return err
		}
	}

	if !loadProjectOpts.SkipCheck {
		if err := cfg.FullCheck(); err != nil {
			return err
		}
	}

	return nil
}

func (e *Executor) cleanupProject() error {
	e.log.Debugln("Cleaning up.")

	if e.cfg != nil {
		for _, plug := range e.cfg.LoadedPlugins() {
			if err := plug.Stop(); err != nil {
				return merry.Errorf("error stopping plugin '%s': %w", plug.Name, err)
			}
		}
	}

	if e.srv != nil {
		e.srv.Stop()
	}

	return nil
}

func (e *Executor) saveLockfile() error {
	lock := e.cfg.Lockfile()

	data, err := yaml.MarshalWithOptions(lock, yaml.UseJSONMarshaler())
	if err != nil {
		return merry.Errorf("marshaling lockfile error: %w", err)
	}

	if err := fileutil.WriteFile(filepath.Join(e.cfg.Dir, config.LockfileName), data, 0o644); err != nil {
		return merry.Errorf("writing lockfile error: %w", err)
	}

	return nil
}

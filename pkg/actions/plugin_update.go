package actions

import (
	"context"
	"fmt"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/pterm/pterm"
)

type PluginUpdate struct {
	log    logger.Logger
	loader *plugins.Loader
	cfg    *config.Project
}

func NewPluginUpdate(log logger.Logger, cfg *config.Project, loader *plugins.Loader) *PluginUpdate {
	return &PluginUpdate{
		log:    log,
		cfg:    cfg,
		loader: loader,
	}
}

func (d *PluginUpdate) Run(ctx context.Context) error {
	prog, _ := d.log.ProgressBar().WithTotal(len(d.cfg.Plugins)).WithTitle("Checking for plugin updates...").Start()
	loadedPlugins := make([]*plugins.Plugin, len(d.cfg.Plugins))

	var updatedPlugins []*config.Plugin

	for i, p := range d.cfg.Plugins {
		prog.UpdateTitle(fmt.Sprintf("Checking for plugin updates: %s", p.Name))

		cur := p.Loaded().Version

		matching, _, err := d.loader.MatchingVersion(ctx, p.Name, p.Source, p.VerConstr())
		if err != nil {
			return err
		}

		if !matching.GreaterThan(cur) {
			loadedPlugins[i] = p.Loaded()

			prog.Increment()

			continue
		}

		// Download new plugin version.
		plug, err := d.loader.DownloadPlugin(ctx, p.Name, p.VerConstr(), p.Source, nil)
		if err != nil {
			return err
		}

		updatedPlugins = append(updatedPlugins, p)
		loadedPlugins[i] = plug

		p.SetLoaded(plug)
		prog.Increment()
	}

	d.cfg.SetLoadedPlugins(loadedPlugins)

	// Print updated plugins info.
	if len(updatedPlugins) == 0 {
		d.log.Println("No updates found.")
	}

	for _, p := range updatedPlugins {
		d.log.Successf("Plugin '%s' updated to %s.\n", p.Name, pterm.Magenta(p.Loaded().Version.String()))
	}

	return nil
}

package actions

import (
	"context"
	"fmt"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/pterm/pterm"
)

type PluginList struct {
	log    logger.Logger
	loader *plugins.Loader
	cfg    *config.Project
}

func NewPluginList(log logger.Logger, cfg *config.Project, loader *plugins.Loader) *PluginList {
	return &PluginList{
		log:    log,
		cfg:    cfg,
		loader: loader,
	}
}

func (d *PluginList) Run(ctx context.Context) error {
	prog, _ := d.log.ProgressBar().WithTotal(len(d.cfg.Plugins)).WithTitle("Checking for plugin updates...").Start()

	data := [][]string{
		{"Name", "Range", "Current", "Wanted", "Latest"},
	}

	for _, p := range d.cfg.Plugins {
		prog.UpdateTitle(fmt.Sprintf("Checking for plugin updates: %s", p.Name))

		matching, latest, err := d.loader.MatchingVersion(ctx, p.Name, p.Source, p.VerConstr())
		if err != nil {
			return err
		}

		cur := p.Loaded().Version

		matchingStr := matching.String()
		if matching.Equal(cur) {
			matchingStr = "-"
		}

		latestStr := latest.String()
		if latest.Equal(cur) {
			latestStr = "-"
		}

		data = append(data, []string{
			pterm.Yellow(p.Name),
			p.Version,
			p.Loaded().Version.String(),
			pterm.Green(matchingStr),
			pterm.Magenta(latestStr),
		})

		prog.Increment()
	}

	return d.log.Table().WithHasHeader().WithData(pterm.TableData(data)).Render()
}

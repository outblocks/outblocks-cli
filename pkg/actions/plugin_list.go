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
}

func NewPluginList(log logger.Logger, loader *plugins.Loader) *PluginList {
	return &PluginList{
		log:    log,
		loader: loader,
	}
}

func (d *PluginList) Run(ctx context.Context, cfg *config.Project) error {
	prog, _ := d.log.ProgressBar().WithTotal(len(cfg.Plugins)).WithTitle("Checking for plugin updates...").Start()

	data := [][]string{
		{"Name", "Range", "Current", "Wanted", "Latest"},
	}

	for _, p := range cfg.Plugins {
		prog.UpdateTitle(fmt.Sprintf("Checking for plugin updates: %s", p.Name))

		matching, latest, err := d.loader.MatchingVersion(ctx, p.Name, p.Source, p.VerRange())
		if err != nil {
			return err
		}

		cur := p.Loaded().Version

		matchingStr := matching.String()
		if matching.EQ(*cur) {
			matchingStr = "-"
		}

		latestStr := latest.String()
		if latest.EQ(*cur) {
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

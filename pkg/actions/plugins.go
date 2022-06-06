package actions

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/pterm/pterm"
)

type PluginManager struct {
	log    logger.Logger
	loader *plugins.Loader
	cfg    *config.Project
}

func NewPluginManager(log logger.Logger, cfg *config.Project, loader *plugins.Loader) *PluginManager {
	return &PluginManager{
		log:    log,
		cfg:    cfg,
		loader: loader,
	}
}

func (m *PluginManager) Update(ctx context.Context) error {
	prog, _ := m.log.ProgressBar().WithTotal(len(m.cfg.Plugins)).WithTitle("Checking for plugin updates...").Start()
	loadedPlugins := make([]*plugins.Plugin, len(m.cfg.Plugins))

	var updatedPlugins []*config.Plugin

	for i, p := range m.cfg.Plugins {
		prog.UpdateTitle(fmt.Sprintf("Checking for plugin updates: %s", p.Name))

		cur := p.Loaded().Version

		matching, _, err := m.loader.MatchingVersion(ctx, p.Name, p.Source, p.VerConstr())
		if err != nil {
			return err
		}

		if !matching.GreaterThan(cur) {
			loadedPlugins[i] = p.Loaded()

			prog.Increment()

			continue
		}

		matchingConstr, _ := semver.NewConstraint(matching.String())

		// Download new plugin version.
		plug, err := m.loader.DownloadPlugin(ctx, p.Name, matchingConstr, p.Source, nil)
		if err != nil {
			return err
		}

		updatedPlugins = append(updatedPlugins, p)
		loadedPlugins[i] = plug

		p.SetLoaded(plug)
		prog.Increment()
	}

	m.cfg.SetLoadedPlugins(loadedPlugins)

	// Print updated plugins info.
	if len(updatedPlugins) == 0 {
		m.log.Println("No updates found.")
	}

	for _, p := range updatedPlugins {
		m.log.Successf("Plugin '%s' updated to %s.\n", p.Name, pterm.Magenta(p.Loaded().Version.String()))
	}

	return nil
}

func (m *PluginManager) List(ctx context.Context) error {
	prog, _ := m.log.ProgressBar().WithTotal(len(m.cfg.Plugins)).WithTitle("Checking for plugin updates...").Start()

	data := [][]string{
		{"Name", "Range", "Current", "Wanted", "Latest"},
	}

	for _, p := range m.cfg.Plugins {
		prog.UpdateTitle(fmt.Sprintf("Checking for plugin updates: %s", p.Name))

		matching, latest, err := m.loader.MatchingVersion(ctx, p.Name, p.Source, p.VerConstr())
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

	return m.log.Table().WithHasHeader().WithData(pterm.TableData(data)).Render()
}

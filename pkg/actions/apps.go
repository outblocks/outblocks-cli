package actions

import (
	"context"
	"fmt"
	"sort"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins/client"
	"github.com/pterm/pterm"
)

type AppManager struct {
	log logger.Logger
	cfg *config.Project
}

func NewAppManager(log logger.Logger, cfg *config.Project) *AppManager {
	return &AppManager{
		log: log,
		cfg: cfg,
	}
}

func (m *AppManager) appsList(ctx context.Context) (data [][]string, err error) {
	yamlContext := &client.YAMLContext{
		Prefix: "$.state",
		Data:   m.cfg.YAMLData(),
	}

	state, _, err := getState(ctx, m.cfg.State, false, 0, true, yamlContext)
	if err != nil {
		return nil, err
	}

	for _, a := range m.cfg.Apps {
		_, ok := state.Apps[a.ID()]
		delete(state.Apps, a.ID())
		deployed := pterm.Green("\u2713")

		if !ok {
			deployed = ""
		}

		data = append(data, []string{
			pterm.Yellow(a.Name()),
			pterm.Magenta(a.Type()),
			deployed,
			pterm.Green(a.DeployPlugin().Name),
			a.Dir(),
		})
	}

	for _, app := range state.Apps {
		data = append(data, []string{
			pterm.Yellow(app.App.Name),
			pterm.Magenta(app.App.Type),
			pterm.Green("\u2713"),
			pterm.Green(app.App.Deploy.Plugin),
			"",
		})
	}

	if len(data) > 0 {
		sort.Slice(data, func(i, j int) bool {
			return fmt.Sprintf("%s:%s", data[i][0], data[i][1]) < fmt.Sprintf("%s:%s", data[j][0], data[j][1])
		})

		data = append([][]string{
			{"Name", "Type", "Deployed", "Plugin", "Local dir"},
		}, data...)

		return data, nil
	}

	return nil, nil
}

func (m *AppManager) List(ctx context.Context) error {
	appList, err := m.appsList(ctx)
	if err != nil {
		return err
	}

	if len(appList) != 0 {
		err := m.log.Table().WithHasHeader().WithData(pterm.TableData(appList)).Render()
		if err != nil {
			return err
		}
	}

	return nil
}

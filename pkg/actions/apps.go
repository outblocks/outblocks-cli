package actions

import (
	"context"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
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

func (m *AppManager) appsList() [][]string {
	data := [][]string{
		{"Name", "Type", "Deployment", "Path"},
	}

	for _, a := range m.cfg.Apps {
		data = append(data, []string{
			pterm.Yellow(a.Name()),
			pterm.Magenta(a.Type()),
			pterm.Green(a.DeployPlugin().Name),
			a.Dir(),
		})
	}

	if len(data) > 1 {
		return data
	}

	return nil
}

func (m *AppManager) List(ctx context.Context) error {
	appList := m.appsList()

	if len(appList) != 0 {
		err := m.log.Table().WithHasHeader().WithData(pterm.TableData(appList)).Render()
		if err != nil {
			return err
		}
	}

	return nil
}

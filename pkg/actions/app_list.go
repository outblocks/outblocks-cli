package actions

import (
	"context"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/pterm/pterm"
)

type AppList struct {
	log  logger.Logger
	cfg  *config.Project
	opts *AppListOptions
}

type AppListOptions struct{}

func NewAppList(log logger.Logger, cfg *config.Project, opts *AppListOptions) *AppList {
	return &AppList{
		log:  log,
		cfg:  cfg,
		opts: opts,
	}
}

func (d *AppList) appsList() [][]string {
	data := [][]string{
		{"Name", "Type", "Deployment", "Path"},
	}

	for _, a := range d.cfg.Apps {
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

func (d *AppList) Run(ctx context.Context) error {
	appList := d.appsList()

	if len(appList) != 0 {
		err := d.log.Table().WithHasHeader().WithData(pterm.TableData(appList)).Render()
		if err != nil {
			return err
		}
	}

	return nil
}

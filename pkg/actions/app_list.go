package actions

import (
	"context"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/pterm/pterm"
)

type AppList struct {
	log  logger.Logger
	opts *AppListOptions
}

type AppListOptions struct{}

func NewAppList(log logger.Logger, opts *AppListOptions) *AppList {
	return &AppList{
		log:  log,
		opts: opts,
	}
}

func (d *AppList) appsList(cfg *config.Project) [][]string {
	data := [][]string{
		{"Name", "Type", "Deployment", "Path"},
	}

	for _, a := range cfg.Apps {
		data = append(data, []string{
			pterm.Yellow(a.Name()),
			pterm.Magenta(a.Type()),
			pterm.Green(a.DeployPlugin().Name),
			a.Path(),
		})
	}

	if len(data) > 1 {
		return data
	}

	return nil
}

func (d *AppList) Run(ctx context.Context, cfg *config.Project) error {
	appList := d.appsList(cfg)

	if len(appList) != 0 {
		err := d.log.Table().WithHasHeader().WithData(pterm.TableData(appList)).Render()
		if err != nil {
			return err
		}
	}

	return nil
}

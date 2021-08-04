package actions

import (
	"context"
	"fmt"
	"os"

	"github.com/otiai10/copy"
	"github.com/outblocks/outblocks-cli/pkg/clipath"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/txn2/txeh"
)

type Run struct {
	log  logger.Logger
	cfg  *config.Project
	opts *RunOptions

	hosts      *txeh.Hosts
	addedHosts []string
}

type RunOptions struct {
	LocalIP     string
	HostsSuffix string
	AddHosts    bool
}

func NewRun(log logger.Logger, cfg *config.Project, opts *RunOptions) *Run {
	return &Run{
		log:  log,
		cfg:  cfg,
		opts: opts,
	}
}

func (d *Run) cleanup() error {
	if len(d.addedHosts) > 0 {
		d.hosts.RemoveHosts(d.addedHosts)

		return d.hosts.Save()
	}

	return nil
}

func (d *Run) AddHosts(hosts ...string) {
	d.addedHosts = append(d.addedHosts, hosts...)
	d.hosts.AddHosts(d.opts.LocalIP, hosts)
}

func (d *Run) init() error {
	var err error

	d.hosts, err = txeh.NewHostsDefault()
	if err != nil {
		return err
	}

	backupHosts := clipath.DataPath("hosts.original")
	if _, err := os.Stat(backupHosts); os.IsNotExist(err) {
		if err = copy.Copy(d.hosts.WriteFilePath, backupHosts); err != nil {
			return fmt.Errorf("cannot backup hosts file: %w", err)
		}
	}

	return err
}

func prepareRunMap(cfg *config.Project) map[*plugins.Plugin]*plugin_go.RunRequest {
	runMap := make(map[*plugins.Plugin]*plugin_go.RunRequest)

	for _, app := range cfg.Apps {
		runPlugin := app.RunPlugin()
		appType := app.PluginType()

		if _, ok := runMap[runPlugin]; !ok {
			runMap[runPlugin] = &plugin_go.RunRequest{}
		}

		runMap[runPlugin].Apps = append(runMap[runPlugin].Apps, appType)
	}

	for _, dep := range cfg.Dependencies {
		runPlugin := dep.RunPlugin()
		depType := dep.PluginType()

		if _, ok := runMap[runPlugin]; !ok {
			runMap[runPlugin] = &plugin_go.RunRequest{}
		}

		runMap[runPlugin].Dependencies = append(runMap[runPlugin].Dependencies, depType)
	}

	return runMap
}

func run(_ context.Context, runMap map[*plugins.Plugin]*plugin_go.RunRequest) (map[*plugins.Plugin]*plugin_go.RunDoneResponse, error) { // nolint: unparam
	retMap := make(map[*plugins.Plugin]*plugin_go.RunDoneResponse)

	for plug, req := range runMap {
		fmt.Println("RUN", plug.Name, req)
	}

	return retMap, nil
}

func (d *Run) Run(ctx context.Context) error {
	err := d.init()
	if err != nil {
		return err
	}

	spinner, _ := d.log.Spinner().WithRemoveWhenDone(true).Start("Preparing...")

	runMap := prepareRunMap(d.cfg)
	runRetMap, err := run(ctx, runMap)

	_ = spinner.Stop()

	if err != nil {
		return err
	}

	fmt.Println(runMap, runRetMap)

	// TODO: RUN
	// d.hosts.AddHosts("127.0.0.1", []string{"test_me.local.test"})

	// fmt.Println(d.hosts.RenderHostsFile())

	return d.cleanup()
}

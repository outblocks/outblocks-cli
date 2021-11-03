package run

import (
	"bufio"
	"fmt"
	"sync"

	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/config"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

type LocalApp struct {
	*types.AppRun
}

type LocalAppRunInfo struct {
	*util.CmdInfo
	*LocalApp
	wg sync.WaitGroup
}

func NewLocalAppRunInfo(a *LocalApp) (*LocalAppRunInfo, error) {
	info := &LocalAppRunInfo{
		LocalApp: a,
	}

	var err error

	info.CmdInfo, err = util.NewCmdInfo(
		a.App.Properties["run"].(*config.AppRun).Command,
		a.App.Dir,
		util.FlattenEnvMap(a.App.Env),
	)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (a *LocalAppRunInfo) Run(outputCh chan<- *plugin_go.RunOutputResponse) error {
	err := a.CmdInfo.Run()
	if err != nil {
		return err
	}

	a.wg.Add(2)

	go func() {
		s := bufio.NewScanner(a.Stdout())
		for s.Scan() {
			out := &plugin_go.RunOutputResponse{
				Source:  plugin_go.RunOutpoutSourceApp,
				ID:      a.App.ID,
				Name:    a.App.Name,
				Message: s.Text(),
			}

			outputCh <- out
		}

		a.wg.Done()
	}()

	go func() {
		s := bufio.NewScanner(a.Stderr())
		for s.Scan() {
			out := &plugin_go.RunOutputResponse{
				Source:   plugin_go.RunOutpoutSourceApp,
				ID:       a.App.ID,
				Name:     a.App.Name,
				Message:  s.Text(),
				IsStderr: true,
			}

			outputCh <- out
		}

		a.wg.Done()
	}()

	return nil
}

func (a *LocalAppRunInfo) Stop() error {
	err := a.CmdInfo.Stop()

	a.wg.Wait()

	return err
}

func (a *LocalAppRunInfo) Wait() error {
	err := a.CmdInfo.Wait()
	if err == nil {
		err = fmt.Errorf("exited")
	}

	a.wg.Wait()

	return err
}

func (a *LocalApp) Run(outputCh chan<- *plugin_go.RunOutputResponse) (*LocalAppRunInfo, error) {
	i, err := NewLocalAppRunInfo(a)
	if err != nil {
		return nil, err
	}

	err = i.Run(outputCh)

	return i, err
}

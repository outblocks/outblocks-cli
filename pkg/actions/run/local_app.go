package run

import (
	"bufio"
	"fmt"
	"sync"

	"github.com/outblocks/outblocks-cli/internal/util"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
)

type LocalApp struct {
	*apiv1.AppRun
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
		a.App.Run.Command,
		a.App.Dir,
		util.FlattenEnvMap(a.App.Env),
	)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (a *LocalAppRunInfo) Run(outputCh chan<- *apiv1.RunOutputResponse) error {
	err := a.CmdInfo.Run()
	if err != nil {
		return err
	}

	a.wg.Add(2)

	go func() {
		s := bufio.NewScanner(a.Stdout())
		for s.Scan() {
			out := &apiv1.RunOutputResponse{
				Source:  apiv1.RunOutputResponse_SOURCE_APP,
				Stream:  apiv1.RunOutputResponse_STREAM_STDOUT,
				Id:      a.App.Id,
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
			out := &apiv1.RunOutputResponse{
				Source:  apiv1.RunOutputResponse_SOURCE_APP,
				Stream:  apiv1.RunOutputResponse_STREAM_STDERR,
				Id:      a.App.Id,
				Name:    a.App.Name,
				Message: s.Text(),
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

func (a *LocalApp) Run(outputCh chan<- *apiv1.RunOutputResponse) (*LocalAppRunInfo, error) {
	i, err := NewLocalAppRunInfo(a)
	if err != nil {
		return nil, err
	}

	err = i.Run(outputCh)

	return i, err
}

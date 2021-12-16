package run

import (
	"bufio"
	"sync"
	"time"

	"github.com/outblocks/outblocks-cli/internal/util"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/util/command"
)

type LocalApp struct {
	*apiv1.AppRun
}

type LocalAppRunInfo struct {
	*command.Cmd
	*LocalApp
	wg sync.WaitGroup
}

const (
	localAppCleanupTimeout = 10 * time.Second
)

func NewLocalAppRunInfo(a *LocalApp) (*LocalAppRunInfo, error) {
	info := &LocalAppRunInfo{
		LocalApp: a,
	}

	var err error

	info.Cmd, err = command.New(
		a.App.Run.Command,
		command.WithDir(a.App.Dir),
		command.WithEnv(util.FlattenEnvMap(a.App.Env)),
	)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (a *LocalAppRunInfo) Run(outputCh chan<- *apiv1.RunOutputResponse) error {
	err := a.Cmd.Run()
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
	err := a.Cmd.Stop(localAppCleanupTimeout)

	a.wg.Wait()

	return err
}

func (a *LocalAppRunInfo) Wait() error {
	err := a.Cmd.Wait()

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

package run

import (
	"context"
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

type LocalRunResult struct {
	Apps     map[string]*LocalAppRunInfo
	Deps     map[string]*LocalDependencyRunInfo
	OutputCh chan *plugin_go.RunOutputResponse
}

func (l *LocalRunResult) Stop() error {
	var firstErr error

	for _, a := range l.Apps {
		err := a.Stop()
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	close(l.OutputCh)

	return firstErr
}

func (l *LocalRunResult) Wait() error {
	errCh := make(chan error, 1)
	total := len(l.Apps)

	for _, a := range l.Apps {
		a := a

		go func() {
			err := a.Wait()
			if err != nil {
				err = fmt.Errorf("app %s %w", a.App.Name, err)
			}

			errCh <- err
		}()
	}

	var err error

	for i := 0; i < total; i++ {
		err = <-errCh
		if err != nil {
			break
		}
	}

	return err
}

func RunLocal(ctx context.Context, localApps []*LocalApp, localDeps []*LocalDependency) (*LocalRunResult, error) {
	ret := &LocalRunResult{
		Apps:     make(map[string]*LocalAppRunInfo),
		Deps:     make(map[string]*LocalDependencyRunInfo),
		OutputCh: make(chan *plugin_go.RunOutputResponse),
	}

	for _, app := range localApps {
		info, err := app.Run(ret.OutputCh)
		if err != nil {
			return nil, err
		}

		ret.Apps[app.App.ID] = info
	}

	for _, dep := range localDeps {
		info, err := dep.Run()
		if err != nil {
			return nil, err
		}

		go func() {
			for {
				out := &plugin_go.RunOutputResponse{
					Source: plugin_go.RunOutpoutSourceDependency,
					ID:     dep.Dependency.ID,
					Name:   dep.Dependency.Name,
				}

				select {
				case msg := <-info.StdoutCh:
					out.Message = msg
				case msg := <-info.StderrCh:
					out.Message = msg
					out.IsStderr = true
				case <-info.done:
					return
				}

				ret.OutputCh <- out
			}
		}()

		ret.Deps[dep.Dependency.ID] = info
	}

	return ret, nil
}
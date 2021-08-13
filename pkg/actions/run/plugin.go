package run

import (
	"context"
	"fmt"

	"github.com/outblocks/outblocks-cli/pkg/plugins"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

type PluginRunResult struct {
	Info     map[*plugins.Plugin]*PluginInfo
	OutputCh chan *plugin_go.RunOutputResponse
}

type PluginInfo struct {
	Response *plugin_go.RunningResponse
	done     chan struct{}
	err      error
}

func (i *PluginInfo) Wait() error {
	<-i.done

	return i.err
}

func RunPlugin(ctx context.Context, runMap map[*plugins.Plugin]*plugin_go.RunRequest) (*PluginRunResult, error) {
	ret := &PluginRunResult{
		Info:     make(map[*plugins.Plugin]*PluginInfo),
		OutputCh: make(chan *plugin_go.RunOutputResponse),
	}

	errCh := make(chan error, 1)

	for plug, req := range runMap {
		res, err := plug.Client().Run(ctx, req.Apps, req.Dependencies, nil, ret.OutputCh, errCh)
		if err != nil {
			return nil, err
		}

		i := &PluginInfo{
			Response: res,
			done:     make(chan struct{}),
		}

		go func() {
			err := <-errCh
			if err != nil {
				i.err = fmt.Errorf("plugin:%s: %s", plug.Name, err)
			} else {
				i.err = fmt.Errorf("plugin:%s: exited", plug.Name)
			}

			close(i.done)
		}()

		ret.Info[plug] = i
	}

	return ret, nil
}

func (l *PluginRunResult) Stop() error {
	var firstErr error

	for p := range l.Info {
		err := p.Stop()
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	close(l.OutputCh)

	return firstErr
}

func (l *PluginRunResult) Wait() error {
	errCh := make(chan error, 1)
	total := len(l.Info)

	for _, pi := range l.Info {
		pi := pi

		go func() {
			err := pi.Wait()
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

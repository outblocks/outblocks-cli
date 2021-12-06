package run

import (
	"context"
	"fmt"

	"github.com/outblocks/outblocks-cli/pkg/plugins"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
)

type PluginRunResult struct {
	Info     map[*plugins.Plugin]*PluginInfo
	OutputCh chan *apiv1.RunOutputResponse
}

type PluginInfo struct {
	Response *apiv1.RunStartResponse
	done     chan struct{}
	err      error
}

func (i *PluginInfo) Wait() error {
	<-i.done

	return i.err
}

func ThroughPlugin(ctx context.Context, runMap map[*plugins.Plugin]*apiv1.RunRequest) (*PluginRunResult, error) {
	ret := &PluginRunResult{
		Info:     make(map[*plugins.Plugin]*PluginInfo),
		OutputCh: make(chan *apiv1.RunOutputResponse),
	}

	errCh := make(chan error, 1)

	for plug, req := range runMap {
		res, err := plug.Client().Run(ctx, req, ret.OutputCh, errCh)
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

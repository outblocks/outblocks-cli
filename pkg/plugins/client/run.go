package client

import (
	"context"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) Run(ctx context.Context, apps []*types.AppRun, deps []*types.DependencyRun, args map[string]interface{},
	outCh chan<- *plugin_go.RunOutputResponse, errCh chan<- error) (ret *plugin_go.RunningResponse, err error) {
	stream, err := c.lazyStartBiDi(ctx, &plugin_go.RunRequest{
		Apps:         apps,
		Dependencies: deps,
		Args:         args,
	})

	if err != nil && !IsPluginError(err) {
		err = NewPluginError(c, "run error", err)
	}

	close(outCh)

	// if err != nil {
	// 	return nil, err
	// }

	// for {
	// 	res, err := stream.Recv()
	// 	if err == io.EOF {
	// 		break
	// 	}

	// 	if err != nil {
	// 		_ = stream.Close()
	// 		return ret, NewPluginError(c, "apply error", err)
	// 	}

	// 	switch r := res.(type) {
	// 	case *plugin_go.ApplyResponse:
	// 		if callback != nil {
	// 			for _, act := range r.Actions {
	// 				callback(act)
	// 			}
	// 		}
	// 	case *plugin_go.ApplyDoneResponse:
	// 		ret = r
	// 	default:
	// 		return ret, NewPluginError(c, "unexpected response to apply request", err)
	// 	}
	// }

	if ret == nil {
		return nil, NewPluginError(c, "empty run response", nil)
	}

	return ret, stream.DrainAndClose()
}

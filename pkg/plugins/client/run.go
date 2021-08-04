package client

import (
	"context"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) Run(ctx context.Context, apps []*types.App, deps []*types.Dependency) (ret *plugin_go.RunDoneResponse, err error) {
	stream, err := c.lazyStartBiDi(ctx, &plugin_go.RunRequest{
		Apps:         apps,
		Dependencies: deps,
	})

	if err != nil && !IsPluginError(err) {
		err = NewPluginError(c, "run error", err)
	}

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

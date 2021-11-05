package client

import (
	"context"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) Apply(ctx context.Context, state *types.StateData, apps []*types.AppPlan, deps []*types.DependencyPlan, targetApps, skipApps []string, args map[string]interface{}, destroy bool, callback func(*types.ApplyAction)) (ret *plugin_go.ApplyDoneResponse, err error) {
	stream, err := c.lazyStartBiDi(ctx, &plugin_go.ApplyRequest{
		DeployBaseRequest: plugin_go.DeployBaseRequest{
			Apps:         apps,
			Dependencies: deps,
			TargetApps:   targetApps,
			SkipApps:     skipApps,
			Args:         args,

			StateMap: state.PluginsMap[c.name],
			Destroy:  destroy,
		},
	})

	if err != nil {
		if !IsPluginError(err) {
			err = NewPluginError(c, "apply error", err)
		}

		return nil, err
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			_ = stream.Close()

			return ret, NewPluginError(c, "apply error", err)
		}

		switch r := res.(type) {
		case *plugin_go.ApplyResponse:
			if callback != nil {
				for _, act := range r.Actions {
					callback(act)
				}
			}
		case *plugin_go.ApplyDoneResponse:
			return r, stream.DrainAndClose()
		default:
			_ = stream.Close()
			return ret, NewPluginError(c, "unexpected response to apply request", err)
		}
	}
}

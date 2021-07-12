package client

import (
	"context"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) Apply(ctx context.Context, state *types.StateData, apps []*types.AppPlan, deps []*types.DependencyPlan, destroy bool, callback func(*types.ApplyAction)) (ret *plugin_go.ApplyDoneResponse, err error) {
	in, out, err := c.lazyStartBiDi(ctx, &plugin_go.ApplyRequest{
		Apps:         apps,
		Dependencies: deps,

		PluginMap:        state.PluginsMap[c.name],
		AppStates:        state.AppStates,
		DependencyStates: state.DependencyStates,
		Destroy:          destroy,
	})

	if err != nil && !IsPluginError(err) {
		err = NewPluginError(c, "apply error", err)
	}

	if err != nil {
		return nil, err
	}

	close(in)

	for res := range out {
		if res.Error != nil {
			return ret, NewPluginError(c, "apply error", res.Error)
		}

		switch r := res.Response.(type) {
		case *plugin_go.ApplyResponse:
			if callback != nil {
				for _, act := range r.Actions {
					callback(act)
				}
			}
		case *plugin_go.ApplyDoneResponse:
			ret = r
		default:
			return ret, NewPluginError(c, "unexpected response to apply", res.Error)
		}
	}

	if ret == nil {
		return nil, NewPluginError(c, "empty apply response", nil)
	}

	return ret, nil
}

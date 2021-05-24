package client

import (
	"context"
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) Apply(ctx context.Context, pluginState types.PluginStateMap, appStates map[string]*types.AppState, depStates map[string]*types.DependencyState, deployPlan, dnsPlan *types.Plan) (ret *plugin_go.ApplyDoneResponse, err error) {
	in, out, err := c.lazyStartBiDi(ctx, &plugin_go.ApplyRequest{
		PluginMap:        pluginState,
		AppStates:        appStates,
		DependencyStates: depStates,
		DeployPlan:       deployPlan,
		DNSPlan:          dnsPlan,
	})
	if err != nil {
		return nil, err
	}

	close(in)

	for res := range out {
		if res.Error != nil {
			return nil, res.Error
		}

		fmt.Println("DEBUG: CALLBACK APPLY", res.Response)

		switch r := res.Response.(type) {
		case *plugin_go.ApplyResponse:
			// TODO: handle apply response
			fmt.Println("APPLYRES", r)
		case *plugin_go.ApplyDoneResponse:
			// TODO: handle apply done response
			ret = r
			fmt.Println("APPLYDONE", r)
		default:
			return nil, fmt.Errorf("unexpected response to apply request")
		}
	}

	return ret, nil
}

package client

import (
	"context"
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) Plan(ctx context.Context, state *types.StateData, apps []*types.AppPlan, deps []*types.DependencyPlan, verify, destroy bool) (ret *plugin_go.PlanResponse, err error) {
	err = c.lazySendReceive(ctx, &plugin_go.PlanRequest{
		Apps:         apps,
		Dependencies: deps,

		PluginMap:        state.PluginsMap[c.name],
		AppStates:        state.AppStates,
		DependencyStates: state.DependencyStates,
		Verify:           verify,
		Destroy:          destroy,
	},
		func(res plugin_go.Response) error {
			switch r := res.(type) {
			case *plugin_go.PlanResponse:
				ret = r

			default:
				return fmt.Errorf("unexpected response to plan request")
			}

			return nil
		})

	if err != nil && !IsPluginError(err) {
		err = NewPluginError(c, "plan error", err)
	}

	return ret, err
}

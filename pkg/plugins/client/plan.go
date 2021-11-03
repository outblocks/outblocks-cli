package client

import (
	"context"
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) Plan(ctx context.Context, state *types.StateData, apps []*types.AppPlan, deps []*types.DependencyPlan, targetApps, skipApps []string, args map[string]interface{}, verify, destroy bool) (ret *plugin_go.PlanResponse, err error) {
	err = c.lazySendReceive(ctx, &plugin_go.PlanRequest{
		DeployBaseRequest: plugin_go.DeployBaseRequest{
			Apps:         apps,
			Dependencies: deps,
			TargetApps:   targetApps,
			SkipApps:     skipApps,
			Args:         args,

			StateMap: state.PluginsMap[c.name],
			Destroy:  destroy,
		},
		Verify: verify,
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

	if err == nil && ret == nil {
		return nil, NewPluginError(c, "empty plan response", nil)
	}

	return ret, err
}

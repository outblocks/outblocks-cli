package client

import (
	"context"
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) Plan(ctx context.Context, state types.PluginStateMap, apps []*types.AppInfo, deps []*types.DependencyInfo, verify bool) (ret *plugin_go.PlanResponse, err error) {
	err = c.lazySendReceive(ctx, &plugin_go.PlanRequest{
		Apps: apps, Dependencies: deps,
		PluginState: state,
		Verify:      verify,
	},
		func(res *ResponseWithHeader) error {
			fmt.Println("DEBUG: CALLBACK PLAN", res.Response)

			switch r := res.Response.(type) {
			case *plugin_go.PlanResponse:
				ret = r
			default:
				return fmt.Errorf("unexpected response")
			}

			return nil
		})

	return
}

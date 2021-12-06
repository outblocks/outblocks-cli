package client

import (
	"context"

	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func (c *Client) Plan(ctx context.Context, state *types.StateData, apps []*apiv1.AppPlan, deps []*apiv1.DependencyPlan, args map[string]interface{}, verify, destroy bool) (*apiv1.PlanResponse, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	res, err := c.deployPlugin().Plan(ctx, &apiv1.PlanRequest{
		Apps:         apps,
		Dependencies: deps,
		Args:         plugin_util.MustNewStruct(args),

		State:   state.Plugins[c.name].Proto(),
		Destroy: destroy,
		Verify:  verify,
	})

	return res, c.mapError("plan error", err)
}

func (c *Client) Apply(ctx context.Context, state *types.StateData, apps []*apiv1.AppPlan, deps []*apiv1.DependencyPlan, args map[string]interface{}, destroy bool, callback func(*apiv1.ApplyAction)) (*apiv1.ApplyDoneResponse, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	stream, err := c.deployPlugin().Apply(ctx, &apiv1.ApplyRequest{
		Apps:         apps,
		Dependencies: deps,
		Args:         plugin_util.MustNewStruct(args),

		State:   state.Plugins[c.name].Proto(),
		Destroy: destroy,
	})

	if err != nil {
		return nil, c.mapError("apply error", err)
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			return nil, c.mapError("apply error", err)
		}

		if r := res.GetAction(); r != nil {
			if callback != nil {
				for _, act := range r.Actions {
					callback(act)
				}
			}

			continue
		}

		if r := res.GetDone(); r != nil {
			return r, nil
		}

		return nil, c.newPluginError("unexpected response to apply request", err)
	}
}

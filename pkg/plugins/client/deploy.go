package client

import (
	"context"

	"github.com/ansel1/merry/v2"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func (c *Client) Plan(ctx context.Context, state *types.StateData, apps []*apiv1.AppPlan, deps []*apiv1.DependencyPlan, domains []*apiv1.DomainInfo, args map[string]interface{}, verify, destroy bool) (*apiv1.PlanResponse, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	pluginState := state.Plugins[c.name]
	if pluginState == nil {
		pluginState = &types.PluginState{}
	}

	res, err := c.deployPlugin().Plan(ctx, &apiv1.PlanRequest{
		Apps:         apps,
		Dependencies: deps,
		Domains:      domains,
		Args:         plugin_util.MustNewStruct(args),

		State:   pluginState.Proto(),
		Destroy: destroy,
		Verify:  verify,
	})

	return res, c.mapError("plan error", merry.Wrap(err))
}

func (c *Client) Apply(ctx context.Context, state *types.StateData, apps []*apiv1.AppPlan, deps []*apiv1.DependencyPlan, domains []*apiv1.DomainInfo, args map[string]interface{}, destroy bool, callback func(*apiv1.ApplyAction)) (*apiv1.ApplyDoneResponse, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	stream, err := c.deployPlugin().Apply(ctx, &apiv1.ApplyRequest{
		Apps:         apps,
		Dependencies: deps,
		Domains:      domains,
		Args:         plugin_util.MustNewStruct(args),

		State:   state.Plugins[c.name].Proto(),
		Destroy: destroy,
	})

	if err != nil {
		return nil, c.mapError("apply error", merry.Wrap(err))
	}

	var done *apiv1.ApplyDoneResponse

	for {
		res, err := stream.Recv()
		if err != nil {
			return done, c.mapError("apply error", merry.Wrap(err))
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
			done = r

			continue
		}

		return nil, c.newPluginError("unexpected response to apply request", merry.Wrap(err))
	}
}

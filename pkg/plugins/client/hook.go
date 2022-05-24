package client

import (
	"context"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/statefile"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func (c *Client) DeployHook(ctx context.Context, stage apiv1.DeployHookRequest_Stage, state *statefile.StateData, apps []*apiv1.AppPlan, deps []*apiv1.DependencyPlan, args map[string]interface{}, verify, destroy bool) (*apiv1.DeployHookResponse, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	pluginState := state.Plugins[c.name]
	if pluginState == nil {
		pluginState = &statefile.PluginState{}
	}

	res, err := c.deployHook().DeployHook(ctx, &apiv1.DeployHookRequest{
		Stage:        stage,
		Apps:         apps,
		Dependencies: deps,
		Domains:      state.DomainsInfo,
		Args:         plugin_util.MustNewStruct(args),

		State:   pluginState.Proto(),
		Destroy: destroy,
		Verify:  verify,
	})

	return res, c.mapError("deploy hook error", merry.Wrap(err))
}

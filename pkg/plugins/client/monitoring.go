package client

import (
	"context"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/statefile"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func (c *Client) PlanMonitoring(ctx context.Context, state *statefile.StateData, monitoring *apiv1.MonitoringData, args map[string]interface{}, verify, destroy bool) (*apiv1.PlanMonitoringResponse, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	pluginState := state.Plugins[c.name]
	if pluginState == nil {
		pluginState = &statefile.PluginState{}
	}

	res, err := c.monitoringPlugin().PlanMonitoring(ctx, &apiv1.PlanMonitoringRequest{
		Data: monitoring,
		Args: plugin_util.MustNewStruct(args),

		State:   pluginState.Proto(),
		Destroy: destroy,
		Verify:  verify,
	})

	return res, c.mapError("plan monitoring error", merry.Wrap(err))
}

func (c *Client) ApplyMonitoring(ctx context.Context, state *statefile.StateData, monitoring *apiv1.MonitoringData, args map[string]interface{}, destroy bool, callback func(*apiv1.ApplyAction)) (*apiv1.ApplyMonitoringDoneResponse, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	stream, err := c.monitoringPlugin().ApplyMonitoring(ctx, &apiv1.ApplyMonitoringRequest{
		Data: monitoring,
		Args: plugin_util.MustNewStruct(args),

		State:   state.Plugins[c.name].Proto(),
		Destroy: destroy,
	})

	if err != nil {
		return nil, c.mapError("apply monitoring error", merry.Wrap(err))
	}

	var done *apiv1.ApplyMonitoringDoneResponse

	for {
		res, err := stream.Recv()
		if err != nil {
			return done, c.mapError("apply monitoring error", merry.Wrap(err))
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

		return nil, c.newPluginError("unexpected response to apply dns request", merry.Wrap(err))
	}
}

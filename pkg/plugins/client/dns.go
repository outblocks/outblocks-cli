package client

import (
	"context"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/statefile"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func (c *Client) PlanDNS(ctx context.Context, state *statefile.StateData, records []*apiv1.DNSRecord, args map[string]interface{}, verify, destroy bool) (*apiv1.PlanDNSResponse, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	pluginState := state.Plugins[c.name]
	if pluginState == nil {
		pluginState = &statefile.PluginState{}
	}

	res, err := c.dnsPlugin().PlanDNS(ctx, &apiv1.PlanDNSRequest{
		DnsRecords: records,
		Domains:    state.DomainsInfo,
		Args:       plugin_util.MustNewStruct(args),

		State:   pluginState.Proto(),
		Destroy: destroy,
		Verify:  verify,
	})

	return res, c.mapError("plan dns error", merry.Wrap(err))
}

func (c *Client) ApplyDNS(ctx context.Context, state *statefile.StateData, records []*apiv1.DNSRecord, args map[string]interface{}, destroy bool, callback func(*apiv1.ApplyAction)) (*apiv1.ApplyDNSDoneResponse, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	stream, err := c.dnsPlugin().ApplyDNS(ctx, &apiv1.ApplyDNSRequest{
		DnsRecords: records,
		Domains:    state.DomainsInfo,
		Args:       plugin_util.MustNewStruct(args),

		State:   state.Plugins[c.name].Proto(),
		Destroy: destroy,
	})

	if err != nil {
		return nil, c.mapError("apply dns error", merry.Wrap(err))
	}

	var done *apiv1.ApplyDNSDoneResponse

	for {
		res, err := stream.Recv()
		if err != nil {
			return done, c.mapError("apply dns error", merry.Wrap(err))
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

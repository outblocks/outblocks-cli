package client

import (
	"context"
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (c *Client) ForceUnlock(ctx context.Context, typ, env string, props map[string]interface{}, lockinfo string) error {
	return c.lazySendReceive(ctx, &plugin_go.ForceUnlockRequest{
		StateType:  typ,
		Env:        env,
		LockInfo:   lockinfo,
		Properties: props,
	}, func(res *ResponseWithHeader) error {
		switch res.Response.(type) {
		case *plugin_go.EmptyResponse:
		default:
			return fmt.Errorf("unexpected response")
		}

		return nil
	})
}

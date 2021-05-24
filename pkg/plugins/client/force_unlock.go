package client

import (
	"context"
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (c *Client) ReleaseLock(ctx context.Context, typ, env string, props map[string]interface{}, lockID string) error {
	return c.lazySendReceive(ctx, &plugin_go.ReleaseLockRequest{
		StateType:  typ,
		Env:        env,
		LockID:     lockID,
		Properties: props,
	}, func(res *ResponseWithHeader) error {
		switch res.Response.(type) {
		case *plugin_go.EmptyResponse:
		default:
			return fmt.Errorf("unexpected response to release lock request")
		}

		return nil
	})
}

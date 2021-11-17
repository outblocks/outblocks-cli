package client

import (
	"context"
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (c *Client) ReleaseStateLock(ctx context.Context, typ string, props map[string]interface{}, lockinfo string) error {
	err := c.lazySendReceive(ctx, &plugin_go.ReleaseStateLockRequest{
		StateType:  typ,
		LockInfo:   lockinfo,
		Properties: props,
	}, func(res plugin_go.Response) error {
		switch res.(type) {
		case *plugin_go.EmptyResponse:
		default:
			return fmt.Errorf("unexpected response to release lock request")
		}

		return nil
	})

	if err != nil && !IsPluginError(err) {
		err = NewPluginError(c, "release lock error", err)
	}

	return err
}

package client

import (
	"context"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (c *Client) Init(ctx context.Context, props map[string]interface{}) error {
	return c.lazySendReceive(ctx, &plugin_go.InitRequest{Properties: props}, func(res *ResponseWithHeader) error {
		c.log.Debugln("Init callback")

		return nil
	})
}

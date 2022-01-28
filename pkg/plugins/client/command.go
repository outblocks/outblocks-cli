package client

import (
	"context"

	"github.com/ansel1/merry/v2"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
)

func (c *Client) Command(ctx context.Context, req *apiv1.CommandRequest) error {
	if err := c.Start(ctx); err != nil {
		return err
	}

	_, err := c.commandPlugin().Command(ctx, req)

	return c.mapError("command error", merry.Wrap(err))
}

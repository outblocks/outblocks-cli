package client

import (
	"context"

	"github.com/ansel1/merry/v2"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
)

func (c *Client) Logs(ctx context.Context, req *apiv1.LogsRequest, callback func(*apiv1.LogsResponse)) error {
	if err := c.Start(ctx); err != nil {
		return err
	}

	stream, err := c.logsPlugin().Logs(ctx, req)

	if err != nil {
		return c.mapError("logs error", merry.Wrap(err))
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			if ctx.Err() == context.Canceled {
				return nil
			}

			return c.mapError("logs error", merry.Wrap(err))
		}

		callback(res)
	}
}

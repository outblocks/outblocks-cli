package client

import (
	"context"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/util"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"google.golang.org/grpc/codes"
)

func (c *Client) Command(ctx context.Context, req *apiv1.CommandRequest) error {
	if err := c.Start(ctx); err != nil {
		return err
	}

	_, err := c.commandPlugin().Command(ctx, req)

	s, ok := util.StatusFromError(err)
	if ok && s.Code() == codes.Canceled && ctx.Err() == context.Canceled {
		return nil
	}

	return c.mapError("command error", merry.Wrap(err))
}

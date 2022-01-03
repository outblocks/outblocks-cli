package client

import (
	"context"

	"github.com/ansel1/merry/v2"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func (c *Client) Init(ctx context.Context) error {
	var err error

	c.once.init.Do(func() {
		err = c.init(ctx)
		if err != nil {
			return
		}

		_, err = c.basicPlugin().Init(ctx, &apiv1.InitRequest{
			HostAddr: c.hostAddr,
		})
	})

	return c.mapError("init error", merry.Wrap(err))
}

func (c *Client) Start(ctx context.Context) error {
	err := c.Init(ctx)
	if err != nil {
		return err
	}

	c.once.start.Do(func() {
		_, err = c.basicPlugin().Start(ctx, &apiv1.StartRequest{
			Properties: plugin_util.MustNewStruct(c.props),
		})
	})

	return c.mapError("start error", merry.Wrap(err))
}

package client

import (
	"context"

	"github.com/ansel1/merry/v2"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func (c *Client) ProjectInit(ctx context.Context, name string, deployPlugins, runPlugins []string, args map[string]interface{}) (*apiv1.ProjectInitResponse, error) {
	err := c.Init(ctx)
	if err != nil {
		return nil, err
	}

	res, err := c.basicPlugin().ProjectInit(ctx, &apiv1.ProjectInitRequest{
		Name:          name,
		DeployPlugins: deployPlugins,
		RunPlugins:    runPlugins,
		Args:          plugin_util.MustNewStruct(args),
	})

	return res, c.mapError("project init error", merry.Wrap(err))
}

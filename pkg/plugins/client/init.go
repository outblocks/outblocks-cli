package client

import (
	"context"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (c *Client) ProjectInit(ctx context.Context, name string, deployPlugins, runPlugins []string, args map[string]interface{}) (*plugin_go.ProjectInitResponse, error) {
	stream, err := c.lazyStartBiDi(ctx, &plugin_go.ProjectInitRequest{
		Name:          name,
		DeployPlugins: deployPlugins,
		RunPlugins:    runPlugins,
		Args:          args,
	})

	if err != nil {
		if !IsPluginError(err) {
			err = NewPluginError(c, "init error", err)
		}

		return nil, err
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			_ = stream.Close()

			return nil, NewPluginError(c, "init error", err)
		}

		switch r := res.(type) {
		case *plugin_go.ProjectInitResponse:
			return r, stream.Close()
		case *plugin_go.EmptyResponse:
		default:
			_ = stream.Close()
			return nil, NewPluginError(c, "unexpected response to init request", err)
		}
	}
}

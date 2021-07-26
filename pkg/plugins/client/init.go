package client

import (
	"context"
	"io"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (c *Client) Init(ctx context.Context, name string, deployPlugins, runPlugins []string, args map[string]interface{}) (*plugin_go.InitResponse, error) {
	stream, err := c.lazyStartBiDi(ctx, &plugin_go.InitRequest{
		Name:          name,
		DeployPlugins: deployPlugins,
		RunPlugins:    runPlugins,
		Args:          args,
	})

	if err != nil && !IsPluginError(err) {
		err = NewPluginError(c, "init error", err)
	}

	if err != nil {
		return nil, err
	}

	for {
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			_ = stream.Close()
			return nil, NewPluginError(c, "init error", err)
		}

		switch r := res.(type) {
		case *plugin_go.InitResponse:
			return r, nil
		case *plugin_go.EmptyResponse:
		default:
			return nil, NewPluginError(c, "unexpected response to init request", err)
		}
	}

	return nil, stream.DrainAndClose()
}

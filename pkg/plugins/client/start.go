package client

import (
	"context"
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (c *Client) Start(ctx context.Context, yamlContext YAMLContext) error {
	return c.sendReceive(ctx, &plugin_go.StartRequest{Properties: c.props}, func(res *ResponseWithHeader) error {
		switch r := res.Response.(type) {
		case *plugin_go.EmptyResponse:
		case *plugin_go.ValidationErrorResponse:
			return yamlContext.Error(r)
		default:
			return fmt.Errorf("unexpected response")
		}

		return nil
	})
}

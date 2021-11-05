package client

import (
	"context"
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (c *Client) Start(ctx context.Context) error {
	var err error

	c.once.start.Do(func() {
		err = c.lazySendReceive(ctx, &plugin_go.StartRequest{Properties: c.props}, func(res plugin_go.Response) error {
			switch r := res.(type) {
			case *plugin_go.EmptyResponse:
			case *plugin_go.ValidationErrorResponse:
				return c.yamlContext.Error(r)
			default:
				return fmt.Errorf("unexpected response to start request")
			}

			return nil
		})
	})

	if err != nil {
		err = NewPluginError(c, "start error", err)
	}

	return err
}

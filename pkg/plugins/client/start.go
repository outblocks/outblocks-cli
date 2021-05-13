package client

import (
	"fmt"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (c *Client) Start() error {
	return c.sendReceive(&plugin_go.StartRequest{Properties: c.props}, func(res *ResponseWithHeader) error {
		fmt.Println("CALLBACK START", res.Response)

		switch v := res.Response.(type) {
		case *plugin_go.EmptyResponse:
		case *plugin_go.ValidationErrorResponse:
			return fileutil.YAMLError(strings.Join([]string{c.yamlPrefix, v.Path}, "."), v.Error, c.yamlData)
		default:
			return fmt.Errorf("unexpected response")
		}

		return nil
	})
}

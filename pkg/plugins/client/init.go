package client

import (
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (c *Client) Init(props map[string]interface{}) error {
	return c.lazySendReceive(&plugin_go.InitRequest{Properties: props}, func(res *ResponseWithHeader) error {
		fmt.Println("CALLBACK INIT", res)

		return nil
	})
}

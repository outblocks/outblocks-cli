package client

import (
	"fmt"

	comm "github.com/outblocks/outblocks-plugin-go"
)

func (c *Client) Init(props map[string]interface{}) error {
	return c.startOneWay(&comm.InitRequest{Properties: props}, true, func(res ResponseWithHeader) error {
		fmt.Println("aaa")

		return nil
	})
}

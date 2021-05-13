package client

import (
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) Plan(apps []*types.App, deps []*types.Dependency) (ret *plugin_go.PlanResponse, err error) {
	err = c.lazySendReceive(&plugin_go.PlanRequest{Apps: apps, Dependencies: deps}, func(res *ResponseWithHeader) error {
		fmt.Println("CALLBACK PLAN", res.Response)

		switch v := res.Response.(type) {
		case *plugin_go.PlanResponse:
			ret = v
		default:
			return fmt.Errorf("unexpected response")
		}

		return nil
	})

	return
}

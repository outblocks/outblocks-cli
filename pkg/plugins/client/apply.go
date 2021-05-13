package client

import (
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) Apply(apps []*types.AppPlan, deps []*types.DependencyPlan) error {
	in, out, err := c.lazyStartBiDi(&plugin_go.ApplyRequest{Apps: apps, Dependencies: deps})
	if err != nil {
		return err
	}

	close(in)

	for res := range out {
		fmt.Println("CALLBACK APPLY", res.Response)

		switch v := res.Response.(type) {
		case *plugin_go.EmptyResponse:
			// TODO: handle apply response
			fmt.Println(v)
		default:
			return fmt.Errorf("unexpected response")
		}

		return nil
	}

	return err
}

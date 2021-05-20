package client

import (
	"context"
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) Apply(ctx context.Context, plan *types.Plan) (ret *plugin_go.ApplyDoneResponse, err error) {
	in, out, err := c.lazyStartBiDi(ctx, &plugin_go.ApplyRequest{Plan: plan})
	if err != nil {
		return nil, err
	}

	close(in)

	for res := range out {
		fmt.Println("DEBUG: CALLBACK APPLY", res.Response)

		switch r := res.Response.(type) {
		case *plugin_go.ApplyResponse:
			// TODO: handle apply response
			fmt.Println(r)
		case *plugin_go.ApplyDoneResponse:
			// TODO: handle apply done response
			ret = r
			fmt.Println(r)
		default:
			return nil, fmt.Errorf("unexpected response")
		}
	}

	return
}

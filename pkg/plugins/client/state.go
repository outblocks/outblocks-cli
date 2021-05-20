package client

import (
	"context"
	"fmt"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) GetState(ctx context.Context, typ, env string, props map[string]interface{}, lock bool, yamlContext YAMLContext) (ret *plugin_go.GetStateResponse, err error) {
	err = c.lazySendReceive(ctx, &plugin_go.GetStateRequest{StateType: typ, Env: env, Properties: props, Lock: lock}, func(res *ResponseWithHeader) error {
		switch r := res.Response.(type) {
		case *plugin_go.GetStateResponse:
			ret = r

		case *plugin_go.LockErrorResponse:
			return fmt.Errorf("%s, to force unlock run:\nok force-unlock %s", r.Error(), r.LockInfo)
		case *plugin_go.ValidationErrorResponse:
			return yamlContext.Error(r)
		default:
			return fmt.Errorf("unexpected response")
		}

		return nil
	})

	return
}

func (c *Client) SaveState(ctx context.Context, state *types.StateData, lockinfo, typ, env string, props map[string]interface{}) (ret *plugin_go.SaveStateResponse, err error) {
	err = c.lazySendReceive(ctx, &plugin_go.SaveStateRequest{State: state, LockInfo: lockinfo, StateType: typ, Env: env, Properties: props}, func(res *ResponseWithHeader) error {
		fmt.Println("DEBUG: CALLBACK SAVESTATE", res.Response)

		switch r := res.Response.(type) {
		case *plugin_go.SaveStateResponse:
			ret = r
		default:
			return fmt.Errorf("unexpected response")
		}

		return nil
	})

	return
}

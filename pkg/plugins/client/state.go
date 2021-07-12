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
			return fmt.Errorf("unexpected response to get state request")
		}

		return nil
	})

	if err != nil && !IsPluginError(err) {
		err = NewPluginError(c, "get state error", err)
	}

	return
}

func (c *Client) SaveState(ctx context.Context, state *types.StateData, typ, env string, props map[string]interface{}) (ret *plugin_go.SaveStateResponse, err error) {
	err = c.lazySendReceive(ctx, &plugin_go.SaveStateRequest{State: state, StateType: typ, Env: env, Properties: props}, func(res *ResponseWithHeader) error {
		switch r := res.Response.(type) {
		case *plugin_go.SaveStateResponse:
			ret = r
		default:
			return fmt.Errorf("unexpected response to save state request")
		}

		return nil
	})

	return
}

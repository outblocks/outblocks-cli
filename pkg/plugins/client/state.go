package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
	"github.com/outblocks/outblocks-plugin-go/types"
)

func (c *Client) GetState(ctx context.Context, typ string, props map[string]interface{}, lock bool, lockWait time.Duration, yamlContext YAMLContext) (ret *plugin_go.GetStateResponse, err error) {
	err = c.lazySendReceive(ctx, &plugin_go.GetStateRequest{
		StateType:  typ,
		Properties: props,
		Lock:       lock,
		LockWait:   lockWait,
	}, func(res plugin_go.Response) error {
		switch r := res.(type) {
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

	return ret, err
}

func (c *Client) SaveState(ctx context.Context, state *types.StateData, typ string, props map[string]interface{}) (ret *plugin_go.SaveStateResponse, err error) {
	stateData, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	err = c.lazySendReceive(ctx, &plugin_go.SaveStateRequest{State: stateData, StateType: typ, Properties: props}, func(res plugin_go.Response) error {
		switch r := res.(type) {
		case *plugin_go.SaveStateResponse:
			ret = r
		default:
			return fmt.Errorf("unexpected response to save state request")
		}

		return nil
	})

	return
}

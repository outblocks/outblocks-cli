package client

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ansel1/merry/v2"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"google.golang.org/protobuf/types/known/durationpb"
)

func (c *Client) GetState(ctx context.Context, typ string, props map[string]interface{}, lock bool, lockWait time.Duration, yamlContext YAMLContext) (ret *apiv1.GetStateResponse_State, err error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	stream, err := c.statePlugin().GetState(ctx, &apiv1.GetStateRequest{
		StateType:  typ,
		Properties: plugin_util.MustNewStruct(props),
		Lock:       lock,
		LockWait:   durationpb.New(lockWait),
	})
	if err != nil {
		return nil, c.mapError("get state error", merry.Wrap(err))
	}

	var state *apiv1.GetStateResponse_State

	for {
		res, err := stream.Recv()
		if err != nil {
			return state, c.mapErrorWithContext("get state error", merry.Wrap(err), &yamlContext)
		}

		switch r := res.Response.(type) {
		case *apiv1.GetStateResponse_Waiting:
			c.log.Infoln("Lock is acquired. Waiting for it to be free...")
		case *apiv1.GetStateResponse_State_:
			state = r.State
		}
	}
}

func (c *Client) SaveState(ctx context.Context, state *types.StateData, typ string, props map[string]interface{}) (*apiv1.SaveStateResponse, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	stateData, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}

	res, err := c.statePlugin().SaveState(ctx, &apiv1.SaveStateRequest{
		State:      stateData,
		StateType:  typ,
		Properties: plugin_util.MustNewStruct(props),
	})

	return res, c.mapError("save sate error", merry.Wrap(err))
}

func (c *Client) ReleaseStateLock(ctx context.Context, typ string, props map[string]interface{}, lockinfo string) error {
	if err := c.Start(ctx); err != nil {
		return err
	}

	_, err := c.statePlugin().ReleaseStateLock(ctx, &apiv1.ReleaseStateLockRequest{
		StateType:  typ,
		LockInfo:   lockinfo,
		Properties: plugin_util.MustNewStruct(props),
	})

	return c.mapError("release state lock error", merry.Wrap(err))
}

package client

import (
	"context"
	"time"

	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"google.golang.org/protobuf/types/known/durationpb"
)

func (c *Client) AcquireLocks(ctx context.Context, props map[string]interface{}, lockNames []string, lockWait time.Duration) error {
	if err := c.Start(ctx); err != nil {
		return err
	}

	_, err := c.lockingPlugin().AcquireLocks(ctx, &apiv1.AcquireLocksRequest{
		LockNames:  lockNames,
		LockWait:   durationpb.New(lockWait),
		Properties: plugin_util.MustNewStruct(props),
	})

	return c.mapError("acquire locks error", err)
}

func (c *Client) ReleaseLocks(ctx context.Context, props map[string]interface{}, locks map[string]string) error {
	if err := c.Start(ctx); err != nil {
		return err
	}

	_, err := c.lockingPlugin().ReleaseLocks(ctx, &apiv1.ReleaseLocksRequest{
		Locks:      locks,
		Properties: plugin_util.MustNewStruct(props),
	})

	return c.mapError("release locks error", err)
}

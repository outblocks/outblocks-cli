package client

import (
	"context"
	"time"

	"github.com/ansel1/merry/v2"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
	"google.golang.org/protobuf/types/known/durationpb"
)

func (c *Client) AcquireLocks(ctx context.Context, props map[string]interface{}, lockNames []string, lockWait time.Duration, yamlContext *YAMLContext) (map[string]string, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	stream, err := c.lockingPlugin().AcquireLocks(ctx, &apiv1.AcquireLocksRequest{
		LockNames:  lockNames,
		LockWait:   durationpb.New(lockWait),
		Properties: plugin_util.MustNewStruct(props),
	})
	if err != nil {
		return nil, c.mapErrorWithContext("acquire locks error", merry.Wrap(err), yamlContext)
	}

	var locks map[string]string

	for {
		msg, err := stream.Recv()
		if err != nil {
			return locks, c.mapErrorWithContext("acquire locks error", merry.Wrap(err), yamlContext)
		}

		if msg.Waiting {
			c.log.Infoln("Lock is acquired. Waiting for it to be free...")
		}

		if len(msg.Locks) > 0 {
			locks = msg.Locks
		}
	}
}

func (c *Client) ReleaseLocks(ctx context.Context, props map[string]interface{}, locks map[string]string) error {
	if err := c.Start(ctx); err != nil {
		return err
	}

	_, err := c.lockingPlugin().ReleaseLocks(ctx, &apiv1.ReleaseLocksRequest{
		Locks:      locks,
		Properties: plugin_util.MustNewStruct(props),
	})

	return c.mapError("release locks error", merry.Wrap(err))
}

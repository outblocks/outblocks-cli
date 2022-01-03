package client

import (
	"context"
	"io"

	"github.com/ansel1/merry/v2"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
)

func (c *Client) Run(ctx context.Context, req *apiv1.RunRequest,
	outCh chan<- *apiv1.RunOutputResponse, errCh chan<- error) (*apiv1.RunStartResponse, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	stream, err := c.runPlugin().Run(ctx, req)

	if err != nil {
		return nil, c.mapError("run error", merry.Wrap(err))
	}

	res, err := stream.Recv()
	if err != nil {
		return nil, c.mapError("run error", merry.Wrap(err))
	}

	if r := res.GetStart(); r != nil {
		// Seems that everything is running, continue to process messages asynchronously.
		go func() {
			defer func() {
				close(outCh)
				close(errCh)
			}()

			for {
				res, err := stream.Recv()
				if err != nil && ctx.Err() != nil {
					return
				}

				if err == io.EOF {
					errCh <- merry.Prepend(err, "run stopped unexpectedly")
					return
				}

				if err != nil {
					errCh <- merry.Prepend(err, "run error")
					return
				}

				if r := res.GetOutput(); r != nil {
					outCh <- r
					continue
				}

				errCh <- merry.Prepend(err, "unexpected response to run request")

				return
			}
		}()

		return r, nil
	}

	return nil, c.newPluginError("unexpected response to run request", merry.Wrap(err))
}

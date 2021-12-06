package client

import (
	"context"
	"fmt"
	"io"

	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
)

func (c *Client) Run(ctx context.Context, req *apiv1.RunRequest,
	outCh chan<- *apiv1.RunOutputResponse, errCh chan<- error) (*apiv1.RunStartResponse, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	stream, err := c.runPlugin().Run(ctx, req)

	if err != nil {
		return nil, c.mapError("run error", err)
	}

	res, err := stream.Recv()
	if err != nil {
		return nil, c.mapError("run error", err)
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
					errCh <- fmt.Errorf("run stopped unexpectedly")
					return
				}

				if err != nil {
					errCh <- fmt.Errorf("run error: %w", err)
					return
				}

				if r := res.GetOutput(); r != nil {
					outCh <- r
					continue
				}

				errCh <- fmt.Errorf("unexpected response to run request: %w", err)

				return
			}
		}()

		return r, nil
	}

	return nil, c.newPluginError("unexpected response to run request", err)
}

package client

import (
	"context"
	"errors"
	"fmt"
	"io"

	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func (c *Client) Run(ctx context.Context, req *plugin_go.RunRequest,
	outCh chan<- *plugin_go.RunOutputResponse, errCh chan<- error) (ret *plugin_go.RunningResponse, err error) {
	stream, err := c.lazyStartBiDi(ctx, req)

	if err != nil {
		if !IsPluginError(err) {
			err = NewPluginError(c, "run error", err)
		}

		close(outCh)

		return nil, err
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			_ = stream.Close()

			return ret, NewPluginError(c, "run error", err)
		}

		switch r := res.(type) {
		case *plugin_go.RunningResponse:
			// Seems that everything is running, continue to process messages asynchronously.
			go func() {
				defer func() {
					stream.Close()
					close(outCh)
				}()

				for {
					res, err := stream.Recv()
					if err == io.EOF {
						errCh <- fmt.Errorf("run stopped unexpectedly")
						return
					}

					if err != nil {
						errCh <- fmt.Errorf("run error: %w", err)
						return
					}

					switch r := res.(type) {
					case *plugin_go.RunOutputResponse:
						outCh <- r
					default:
						errCh <- fmt.Errorf("unexpected response to run request: %w", err)
						return
					}
				}
			}()

			return r, nil
		default:
			close(outCh)

			_ = stream.Close()

			return nil, NewPluginError(c, "unexpected response to run request", err)
		}
	}

	if ret == nil {
		close(outCh)

		return nil, NewPluginError(c, "empty run response", nil)
	}

	return ret, nil
}

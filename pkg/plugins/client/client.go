package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/outblocks/outblocks-cli/pkg/cli"
	comm "github.com/outblocks/outblocks-plugin-go"
)

type Client struct {
	addr string
}

const (
	defaultTimeout = 10 * time.Second
)

func NewClient(ctx *cli.Context, handshake *comm.Handshake) (*Client, error) {
	if err := ValidateHandshake(handshake); err != nil {
		return nil, fmt.Errorf("invalid handshake: %w", err)
	}

	return &Client{
		addr: handshake.Addr,
	}, nil
}

func (c *Client) connect() (net.Conn, error) {
	return net.DialTimeout("tcp", c.addr, defaultTimeout)
}

func (c *Client) sendRequest(conn net.Conn, req comm.Request) error {
	_ = conn.SetWriteDeadline(time.Now().Add(defaultTimeout))

	// Send header.
	data, err := json.Marshal(&comm.RequestHeader{
		Type: req.Type(),
	})
	if err != nil {
		return fmt.Errorf("header encoding error: %w", err)
	}

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("plugin comm error: %w", err)
	}

	_, err = conn.Write([]byte{'\n'})
	if err != nil {
		return fmt.Errorf("plugin comm error: %w", err)
	}

	// Send request itself.
	data, err = json.Marshal(req)
	if err != nil {
		return fmt.Errorf("request encoding error: %w", err)
	}

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("plugin comm error: %w", err)
	}

	_, err = conn.Write([]byte{'\n'})
	if err != nil {
		return fmt.Errorf("plugin comm error: %w", err)
	}

	return nil
}

type resErr struct {
	ResponseWithHeader
	err error
}

func (c *Client) readResponse(conn net.Conn) <-chan resErr {
	ch := make(chan resErr)

	go func() {
		r := bufio.NewReader(conn)

		for {
			_ = conn.SetReadDeadline(time.Now().Add(defaultTimeout))

			// Read header.
			data, err := r.ReadBytes('\n')
			if err == io.EOF {
				close(ch)
				return
			}

			if err != nil {
				ch <- resErr{err: fmt.Errorf("reading header error: %w", err)}
				close(ch)

				return
			}

			var header comm.ResponseHeader

			err = json.Unmarshal(data, &header)
			if err != nil {
				ch <- resErr{err: fmt.Errorf("header decode error: %w", err)}
				close(ch)

				return
			}

			// Read actual response.
			data, err = r.ReadBytes('\n')
			if err != nil {
				ch <- resErr{err: fmt.Errorf("reading response error: %w", err)}
				close(ch)

				return
			}

			var res comm.Response

			switch header.Type {
			case comm.ResponseTypeData:
				res = &comm.DataResponse{}
			case comm.ResponseTypeEmpty:
				res = &comm.EmptyResponse{}
			case comm.ResponseTypeError:
				res = &comm.ErrorResponse{}
			case comm.ResponseTypeMap:
				res = &comm.MapResponse{}
			case comm.ResponseTypePrompt:
				res = &comm.PromptResponse{}
			case comm.ResponseTypeMessage:
				res = &comm.MessageResponse{}
			case comm.ResponseTypeUnhandled:
				res = &comm.UnhandledResponse{}
			default:
				ch <- resErr{err: fmt.Errorf("unknown response type: %d", header.Type)}
				close(ch)

				return
			}

			err = json.Unmarshal(data, &res)
			if err != nil {
				ch <- resErr{err: fmt.Errorf("response decode error: %w", err)}
				close(ch)

				return
			}

			ch <- resErr{
				ResponseWithHeader: ResponseWithHeader{
					Header:   header,
					Response: res,
				},
			}
		}
	}()

	return ch
}

type ResponseWithHeader struct {
	Header   comm.ResponseHeader
	Response comm.Response
}

func (c *Client) startOneWay(req comm.Request, required bool, callback func(res ResponseWithHeader) error) error {
	in := make(chan comm.Request, 1)
	in <- req
	close(in)

	return c.startBiDi(in, required, callback)
}

func (c *Client) startBiDi(in <-chan comm.Request, required bool, callback func(res ResponseWithHeader) error) error {
	conn, err := c.connect()
	if err != nil {
		return err
	}

	if err := c.sendRequest(conn, <-in); err != nil {
		return err
	}

	ch := c.readResponse(conn)

	handled := false

	for res := range ch {
		fmt.Println("RES", res)

		if res.err != nil {
			return res.err
		}

		handled = handled || res.Header.Type != comm.ResponseTypeUnhandled

		switch res.Header.Type {
		case comm.ResponseTypeMap, comm.ResponseTypeEmpty, comm.ResponseTypeData:
			err = callback(res.ResponseWithHeader)
			if err != nil {
				return err
			}

			// Check for request in queue.
			select {
			case req, ok := <-in:
				if !ok {
					break
				}

				if err := c.sendRequest(conn, req); err != nil {
					return err
				}

			default:
			}

		case comm.ResponseTypePrompt:
			// TODO: handle prompt
		case comm.ResponseTypeUnhandled:
		case comm.ResponseTypeError:
			// TODO: handle error
		case comm.ResponseTypeMessage:
			// TODO: handle message
		default:
		}
	}

	if required && !handled {
		return fmt.Errorf("unhandled request")
	}

	return nil
}

package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/outblocks/outblocks-cli/pkg/logger"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

type Client struct {
	ctx context.Context
	log logger.Logger

	cmd  *exec.Cmd
	addr string

	name        string
	props       map[string]interface{}
	yamlContext YAMLContext

	once sync.Once
}

const (
	defaultTimeout = 10 * time.Second
)

func NewClient(ctx context.Context, log logger.Logger, name string, cmd *exec.Cmd, props map[string]interface{}, yamlContext YAMLContext) (*Client, error) {
	return &Client{
		ctx: ctx,
		log: log,
		cmd: cmd,

		name:        name,
		props:       props,
		yamlContext: yamlContext,
	}, nil
}

func (c *Client) lazyInit(ctx context.Context) error {
	stdoutPipe, err := c.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderrPipe, err := c.cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := c.cmd.Start(); err != nil {
		return err
	}

	// Process handshake.
	stdout := bufio.NewReader(stdoutPipe)
	line, _ := stdout.ReadBytes('\n')

	go func() {
		s := bufio.NewScanner(stderrPipe)
		for s.Scan() {
			c.log.Errorf("plugin '%s' error: %s\n", c.name, s.Text())
		}
	}()

	var handshake *plugin_go.Handshake

	if err := json.Unmarshal(line, &handshake); err != nil {
		return fmt.Errorf("plugin '%s' error: handshake error: %w", c.name, err)
	}

	if handshake == nil {
		return fmt.Errorf("plugin '%s' error: handshake not returned by plugin", c.name)
	}

	if err := ValidateHandshake(handshake); err != nil {
		return fmt.Errorf("plugin '%s' error: invalid handshake: %w", c.name, err)
	}

	c.addr = handshake.Addr

	// Send Start request to validate YAML.
	err = c.Start(ctx, c.yamlContext)
	if err != nil {
		return fmt.Errorf("plugin '%s' start error:\n%s", c.name, err)
	}

	return nil
}

func (c *Client) connect(ctx context.Context) (net.Conn, error) {
	d := &net.Dialer{}

	return d.DialContext(ctx, "tcp", c.addr)
}

func (c *Client) sendRequest(conn net.Conn, req plugin_go.Request) error {
	_ = conn.SetWriteDeadline(time.Now().Add(defaultTimeout))

	// Send header.
	data, err := json.Marshal(&plugin_go.RequestHeader{
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

type ResponseWithError struct {
	*ResponseWithHeader
	Error error
}

func (c *Client) readResponse(conn net.Conn) <-chan *ResponseWithError {
	ch := make(chan *ResponseWithError)

	go func() {
		r := bufio.NewReader(conn)

		for {
			// Read header.
			data, err := r.ReadBytes('\n')
			if err == io.EOF {
				close(ch)
				return
			}

			if err != nil {
				ch <- &ResponseWithError{Error: fmt.Errorf("reading header error: %w", err)}
				close(ch)

				return
			}

			var header plugin_go.ResponseHeader

			err = json.Unmarshal(data, &header)
			if err != nil {
				ch <- &ResponseWithError{Error: fmt.Errorf("header decode error: %w", err)}
				close(ch)

				return
			}

			// Read actual response.
			data, err = r.ReadBytes('\n')
			if err != nil {
				ch <- &ResponseWithError{Error: fmt.Errorf("reading response error: %w", err)}
				close(ch)

				return
			}

			var res plugin_go.Response

			switch header.Type {
			case plugin_go.ResponseTypePlan:
				res = &plugin_go.PlanResponse{}
			case plugin_go.ResponseTypeApply:
				res = &plugin_go.ApplyResponse{}
			case plugin_go.ResponseTypeApplyDone:
				res = &plugin_go.ApplyDoneResponse{}
			case plugin_go.ResponseTypeGetState:
				res = &plugin_go.GetStateResponse{}
			case plugin_go.ResponseTypeLockError:
				res = &plugin_go.LockErrorResponse{}
			case plugin_go.ResponseTypeSaveState:
				res = &plugin_go.SaveStateResponse{}
			case plugin_go.ResponseTypeData:
				res = &plugin_go.DataResponse{}
			case plugin_go.ResponseTypeEmpty:
				res = &plugin_go.EmptyResponse{}
			case plugin_go.ResponseTypeError:
				res = &plugin_go.ErrorResponse{}
			case plugin_go.ResponseTypePrompt:
				res = &plugin_go.PromptResponse{}
			case plugin_go.ResponseTypeMessage:
				res = &plugin_go.MessageResponse{}
			case plugin_go.ResponseTypeUnhandled:
				res = &plugin_go.UnhandledResponse{}
			case plugin_go.ResponseTypeValidationError:
				res = &plugin_go.ValidationErrorResponse{}
			default:
				ch <- &ResponseWithError{Error: fmt.Errorf("unknown response type: %d", header.Type)}
				close(ch)

				return
			}

			err = json.Unmarshal(data, &res)
			if err != nil {
				ch <- &ResponseWithError{Error: fmt.Errorf("response decode error: %w", err)}
				close(ch)

				return
			}

			ch <- &ResponseWithError{
				ResponseWithHeader: &ResponseWithHeader{
					Header:   header,
					Response: res,
				},
			}
		}
	}()

	return ch
}

type ResponseWithHeader struct {
	Header   plugin_go.ResponseHeader
	Response plugin_go.Response
}

func (c *Client) sendReceive(ctx context.Context, req plugin_go.Request, callback func(res *ResponseWithHeader) error) error {
	in, out, err := c.startBiDi(ctx, req)
	if err != nil {
		return err
	}

	return c.handleOneWay(callback, in, out)
}

func (c *Client) lazySendReceive(ctx context.Context, req plugin_go.Request, callback func(res *ResponseWithHeader) error) error {
	in, out, err := c.lazyStartBiDi(ctx, req)
	if err != nil {
		return err
	}

	return c.handleOneWay(callback, in, out)
}

func (c *Client) handleOneWay(callback func(res *ResponseWithHeader) error, in chan<- plugin_go.Request, out <-chan *ResponseWithError) error {
	close(in)

	res := <-out

	if res == nil {
		return fmt.Errorf("unhandled request")
	}

	if res.Error != nil {
		return res.Error
	}

	return callback(res.ResponseWithHeader)
}

func (c *Client) lazyStartBiDi(ctx context.Context, req plugin_go.Request) (in chan<- plugin_go.Request, out <-chan *ResponseWithError, err error) {
	c.once.Do(func() {
		err = c.lazyInit(ctx)
	})

	if err != nil {
		return nil, nil, err
	}

	return c.startBiDi(ctx, req)
}

func (c *Client) startBiDi(ctx context.Context, req plugin_go.Request) (in chan<- plugin_go.Request, out <-chan *ResponseWithError, err error) {
	conn, err := c.connect(ctx)
	if err != nil {
		return nil, nil, err
	}

	if err := c.sendRequest(conn, req); err != nil {
		return nil, nil, err
	}

	inCh := make(chan plugin_go.Request)
	outCh := make(chan *ResponseWithError)
	ch := c.readResponse(conn)

	go func() {
		for res := range ch {
			if res.Error != nil {
				outCh <- res
				close(outCh)

				return
			}

			err = c.handleResponse(conn, inCh, res.ResponseWithHeader, func(res *ResponseWithHeader) error {
				outCh <- &ResponseWithError{ResponseWithHeader: res}

				return nil
			})
			if err != nil {
				outCh <- &ResponseWithError{Error: err}
				close(outCh)

				return
			}
		}

		close(outCh)
	}()

	return inCh, outCh, nil
}

func (c *Client) handleResponse(conn net.Conn, in <-chan plugin_go.Request, res *ResponseWithHeader, callback func(res *ResponseWithHeader) error) error {
	switch r := res.Response.(type) {
	case *plugin_go.PlanResponse,
		*plugin_go.GetStateResponse,
		*plugin_go.SaveStateResponse,
		*plugin_go.EmptyResponse,
		*plugin_go.DataResponse,
		*plugin_go.ValidationErrorResponse,
		*plugin_go.ApplyDoneResponse,
		*plugin_go.ApplyResponse,
		*plugin_go.LockErrorResponse:
		err := callback(res)
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

	case *plugin_go.PromptResponse:
		fmt.Println(r)
		// TODO: handle prompt
	case *plugin_go.ErrorResponse:
		return fmt.Errorf(r.Error)
	case *plugin_go.MessageResponse:
		switch r.Level() {
		case "debug":
			c.log.Debugln(r.Message)
		case "info":
			c.log.Infoln(r.Message)
		case "warn":
			c.log.Warnln(r.Message)
		default:
			c.log.Errorln(r.Message)
		}
	case *plugin_go.UnhandledResponse:
	default:
		panic(fmt.Sprintf("response not handled! type: %d", r.Type()))
	}

	return nil
}

func (c *Client) Stop() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	_ = c.cmd.Process.Signal(syscall.SIGTERM)

	return c.cmd.Wait()
}

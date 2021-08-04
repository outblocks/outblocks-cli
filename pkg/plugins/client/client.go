package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/outblocks/outblocks-cli/pkg/logger"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	plugin_log "github.com/outblocks/outblocks-plugin-go/log"
)

type Client struct {
	log logger.Logger

	cmd  *exec.Cmd
	addr string

	name        string
	props       map[string]interface{}
	yamlContext YAMLContext

	initOnce, startOnce sync.Once
}

const (
	DefaultTimeout = 10 * time.Second
)

func NewClient(log logger.Logger, name string, cmd *exec.Cmd, props map[string]interface{}, yamlContext YAMLContext) (*Client, error) {
	return &Client{
		log: log,
		cmd: cmd,

		name:        name,
		props:       props,
		yamlContext: yamlContext,
	}, nil
}

func (c *Client) lazyInit(_ context.Context) error {
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
			b := s.Bytes()
			if len(b) == 0 {
				continue
			}

			level := b[0]

			prefix := fmt.Sprintf("%s: ", c.name)

			switch plugin_log.Level(level) {
			case plugin_log.LevelError:
				c.log.Errorf("%s%s\n", prefix, string(b[1:]))
			case plugin_log.LevelWarn:
				c.log.Warnf("%s%s': %s\n", prefix, string(b[1:]))
			case plugin_log.LevelInfo:
				c.log.Infof("%s%s\n", prefix, string(b[1:]))
			case plugin_log.LevelDebug:
				c.log.Debugf("%s%s\n", prefix, string(b[1:]))
			case plugin_log.LevelSuccess:
				c.log.Successf("%s%s\n", prefix, string(b[1:]))
			default:
				c.log.Errorf("%s%s\n", prefix, s.Text())
			}
		}
	}()

	var handshake *plugin_go.Handshake

	if err := json.Unmarshal(line, &handshake); err != nil {
		return NewPluginError(c, "handshake error", err)
	}

	if handshake == nil {
		return NewPluginError(c, "handshake not returned by plugin", err)
	}

	if err := ValidateHandshake(handshake); err != nil {
		return NewPluginError(c, "invalid handshake", err)
	}

	c.addr = handshake.Addr

	return nil
}

func (c *Client) connect(ctx context.Context) (net.Conn, error) {
	d := &net.Dialer{}

	return d.DialContext(ctx, "tcp", c.addr)
}

func mapResponseType(header *plugin_go.ResponseHeader) plugin_go.Response {
	switch header.Type {
	case plugin_go.ResponseTypePlan:
		return &plugin_go.PlanResponse{}
	case plugin_go.ResponseTypeApply:
		return &plugin_go.ApplyResponse{}
	case plugin_go.ResponseTypeApplyDone:
		return &plugin_go.ApplyDoneResponse{}
	case plugin_go.ResponseTypeGetState:
		return &plugin_go.GetStateResponse{}
	case plugin_go.ResponseTypeLockError:
		return &plugin_go.LockErrorResponse{}
	case plugin_go.ResponseTypeSaveState:
		return &plugin_go.SaveStateResponse{}
	case plugin_go.ResponseTypeData:
		return &plugin_go.DataResponse{}
	case plugin_go.ResponseTypeEmpty:
		return &plugin_go.EmptyResponse{}
	case plugin_go.ResponseTypeError:
		return &plugin_go.ErrorResponse{}
	case plugin_go.ResponseTypePromptConfirmation:
		return &plugin_go.PromptConfirmation{}
	case plugin_go.ResponseTypePromptInput:
		return &plugin_go.PromptInput{}
	case plugin_go.ResponseTypePromptSelect:
		return &plugin_go.PromptSelect{}
	case plugin_go.ResponseTypeMessage:
		return &plugin_go.MessageResponse{}
	case plugin_go.ResponseTypeUnhandled:
		return &plugin_go.UnhandledResponse{}
	case plugin_go.ResponseTypeValidationError:
		return &plugin_go.ValidationErrorResponse{}
	case plugin_go.ResponseTypeInit:
		return &plugin_go.InitResponse{}
	case plugin_go.ResponseTypeRunDone:
		return &plugin_go.RunDoneResponse{}
	default:
		return nil
	}
}

type ResponseWithHeader struct {
	Header   plugin_go.ResponseHeader
	Response plugin_go.Response
}

func (c *Client) sendReceive(ctx context.Context, req plugin_go.Request, callback func(res plugin_go.Response) error) error {
	stream, err := c.startBiDi(ctx, req)
	if err != nil {
		return err
	}

	return c.handleOneWay(callback, stream)
}

func (c *Client) lazySendReceive(ctx context.Context, req plugin_go.Request, callback func(res plugin_go.Response) error) error {
	stream, err := c.lazyStartBiDi(ctx, req)
	if err != nil {
		return err
	}

	return c.handleOneWay(callback, stream)
}

func (c *Client) handleOneWay(callback func(res plugin_go.Response) error, stream *SenderStream) error {
	res, err := stream.Recv()
	if res == nil {
		_ = stream.Close()
		return fmt.Errorf("unhandled request")
	}

	if err != nil {
		_ = stream.Close()
		return err
	}

	err = callback(res)
	if err != nil {
		_ = stream.Close()
		return err
	}

	return stream.DrainAndClose()
}

func (c *Client) lazyStartBiDi(ctx context.Context, req plugin_go.Request) (*SenderStream, error) {
	var err error

	c.initOnce.Do(func() {
		err = c.lazyInit(ctx)
	})

	if _, ok := req.(*plugin_go.InitRequest); !ok {
		c.startOnce.Do(func() {
			// Send Start request to validate YAML.
			err = c.Start(ctx, c.yamlContext)
			if err != nil {
				err = NewPluginError(c, "start error", err)
			}
		})
	}

	if err != nil {
		return nil, err
	}

	return c.startBiDi(ctx, req)
}

func (c *Client) startBiDi(ctx context.Context, req plugin_go.Request) (*SenderStream, error) {
	conn, err := c.connect(ctx)
	if err != nil {
		return nil, err
	}

	stream := NewSenderStream(c.log, conn)

	if err := stream.Send(req); err != nil {
		return nil, err
	}

	return stream, nil
}

func (c *Client) Stop() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	_ = c.cmd.Process.Signal(syscall.SIGTERM)

	return c.cmd.Wait()
}

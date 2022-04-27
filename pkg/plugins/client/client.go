package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	log logger.Logger

	cmd      *exec.Cmd
	addr     string
	hostAddr string

	conn *grpc.ClientConn

	name        string
	env         string
	props       map[string]interface{}
	yamlContext YAMLContext

	once struct {
		init, start sync.Once
	}
	mu sync.Mutex
}

type YAMLContext struct {
	Prefix string
	Data   []byte
}

const (
	DefaultTimeout = 10 * time.Second
)

func NewClient(log logger.Logger, name, env string, cmd *exec.Cmd, hostAddr string, props map[string]interface{}, yamlContext YAMLContext) (*Client, error) {
	return &Client{
		log:      log,
		cmd:      cmd,
		hostAddr: hostAddr,

		name:        name,
		env:         env,
		props:       props,
		yamlContext: yamlContext,
	}, nil
}

func (c *Client) basicPlugin() apiv1.BasicPluginServiceClient {
	return apiv1.NewBasicPluginServiceClient(c.conn)
}

func (c *Client) statePlugin() apiv1.StatePluginServiceClient {
	return apiv1.NewStatePluginServiceClient(c.conn)
}

func (c *Client) lockingPlugin() apiv1.LockingPluginServiceClient {
	return apiv1.NewLockingPluginServiceClient(c.conn)
}

func (c *Client) deployPlugin() apiv1.DeployPluginServiceClient {
	return apiv1.NewDeployPluginServiceClient(c.conn)
}

func (c *Client) runPlugin() apiv1.RunPluginServiceClient {
	return apiv1.NewRunPluginServiceClient(c.conn)
}

func (c *Client) commandPlugin() apiv1.CommandPluginServiceClient {
	return apiv1.NewCommandPluginServiceClient(c.conn)
}

func (c *Client) logsPlugin() apiv1.LogsPluginServiceClient {
	return apiv1.NewLogsPluginServiceClient(c.conn)
}

func (c *Client) init(ctx context.Context) error {
	c.log.Debugf("Initializing connection to plugin: %s\n", c.name)

	c.mu.Lock()
	defer c.mu.Unlock()

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

		prefix := fmt.Sprintf("%s: ", c.name)

		for s.Scan() {
			b := s.Bytes()

			c.log.Errorf("%s%s\n", prefix, string(b))
		}
	}()

	var handshake *plugin_go.Handshake

	if err := json.Unmarshal(line, &handshake); err != nil {
		return c.newPluginError("handshake error", merry.Wrap(err))
	}

	if handshake == nil {
		return c.newPluginError("handshake not returned by plugin", merry.Wrap(err))
	}

	if err := ValidateHandshake(handshake); err != nil {
		return c.newPluginError("invalid handshake", merry.Wrap(err))
	}

	c.addr = handshake.Addr

	c.conn, err = grpc.DialContext(ctx, c.addr, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd := c.cmd
	c.cmd = nil

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	c.log.Debugf("Stopping client of plugin: %s\n", c.name)

	if c.conn != nil {
		_ = c.conn.Close()
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)

	return cmd.Wait()
}

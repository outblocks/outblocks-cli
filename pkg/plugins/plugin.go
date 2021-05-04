package plugins

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"reflect"
	"runtime"

	"github.com/blang/semver/v4"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
	"github.com/outblocks/outblocks-plugin-go/communication"
)

type Plugin struct {
	Name           string         `json:"name"`
	Author         string         `json:"author"`
	Usage          string         `json:"usage"`
	Description    string         `json:"description"`
	Run            string         `json:"run"`
	Actions        []string       `json:"actions"`
	Hooks          []*PluginHooks `json:"hooks"`
	Supports       []string       `json:"supports"`
	StateTypes     []string       `json:"state_types"`
	SupportedTypes []*PluginType  `json:"supported_types"`

	Path     string `json:"-"`
	yamlPath string
	version  *semver.Version
	source   string
	data     []byte
	actions  []Action
	cmd      *exec.Cmd
	conn     net.Conn
}

type Action int

const (
	ActionDeploy Action = iota
	ActionRun
)

func (p *Plugin) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.Name, validation.Required),
		validation.Field(&p.Actions, validation.Required),
		validation.Field(&p.Hooks),
		validation.Field(&p.SupportedTypes),
	)
}

func (p *Plugin) Locked() *lockfile.Plugin {
	return &lockfile.Plugin{
		Name:    p.Name,
		Version: p.version,
		Source:  p.source,
	}
}

func (p *Plugin) HasAction(a Action) bool {
	for _, act := range p.actions {
		if act == a {
			return true
		}
	}

	return false
}

func mapMatch(m, other map[string]interface{}) bool {
	for k, v := range m {
		v2, ok := other[k]
		if !ok {
			return false
		}

		if !reflect.DeepEqual(v, v2) {
			return false
		}
	}

	return true
}

func (p *Plugin) SupportsType(typ, dep string, other map[string]interface{}) bool {
	for _, t := range p.SupportedTypes {
		if t.Type == typ && (t.Match == nil || ((t.Match.Deploy == "" || t.Match.Deploy == dep) && mapMatch(t.Match.Other, other))) {
			return true
		}
	}

	return false
}

func (p *Plugin) SupportsApp(app string) bool {
	for _, a := range p.Supports {
		if a == app {
			return true
		}
	}

	return false
}

func (p *Plugin) Start(projectPath string) error {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", p.Run)
	} else {
		cmd = exec.Command("sh", "-c", p.Run)
	}

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("OUTBLOCKS_BIN=%s", os.Args[0]),
		fmt.Sprintf("OUTBLOCKS_PLUGIN_DIR=%s", p.Path),
		fmt.Sprintf("OUTBLOCKS_PROJECT_PATH=%s", projectPath),
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	// Process handshake.
	r := bufio.NewReader(stdout)
	line, _ := r.ReadBytes('\n')

	var handshake *communication.Handshake

	if err := json.Unmarshal(line, &handshake); err != nil {
		return fmt.Errorf("handshake error: %w", err)
	}

	if handshake == nil {
		return errorAppend("handshake not returned by plugin", stderr.String())
	}

	if err := handshake.Validate(); err != nil {
		return fmt.Errorf("invalid handshake: %w", err)
	}

	p.cmd = cmd
	p.conn, err = net.Dial("tcp", handshake.Addr)
	// TODO: send init

	return err
}

func errorAppend(msg, stderr string) error {
	if stderr == "" {
		return fmt.Errorf(msg)
	}

	return fmt.Errorf("%s: %s", msg, stderr)
}

func (p *Plugin) Stop() error {
	if p.cmd == nil {
		return nil
	}

	if err := p.cmd.Process.Kill(); err != nil {
		return err
	}

	if p.conn != nil {
		_ = p.conn.Close()
	}

	return p.cmd.Wait()
}

package plugins

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"

	"github.com/blang/semver/v4"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins/client"
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

	Path     string          `json:"-"`
	Version  *semver.Version `json:"-"`
	yamlPath string
	yamlData []byte
	source   string
	actions  []Action
	client   *client.Client
}

type Action int

const (
	ActionDeploy Action = iota
	ActionRun
	ActionDNS
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
		Version: p.Version,
		Source:  p.source,
	}
}

func (p *Plugin) Client() *client.Client {
	return p.client
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

func (p *Plugin) Prepare(ctx context.Context, log logger.Logger, projectName, projectPath string, props map[string]interface{}, yamlPrefix string, yamlData []byte) error {
	var (
		cmd *exec.Cmd
		err error
	)

	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", p.Run)
	} else {
		cmd = exec.Command("sh", "-c", p.Run)
	}

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("OUTBLOCKS_BIN=%s", os.Args[0]),
		fmt.Sprintf("OUTBLOCKS_PLUGIN_DIR=%s", p.Path),
		fmt.Sprintf("OUTBLOCKS_PROJECT_NAME=%s", projectName),
		fmt.Sprintf("OUTBLOCKS_PROJECT_PATH=%s", projectPath),
	)

	p.client, err = client.NewClient(ctx, log, p.Name, cmd, props, client.YAMLContext{
		Prefix: yamlPrefix,
		Data:   yamlData,
	})

	return err
}

func (p *Plugin) Stop() error {
	if p.client == nil {
		return nil
	}

	return p.client.Stop()
}

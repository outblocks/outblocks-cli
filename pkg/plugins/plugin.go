package plugins

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"runtime"

	"github.com/Masterminds/semver"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins/client"
	"github.com/outblocks/outblocks-plugin-go/util/command"
)

type Plugin struct {
	Name           string                            `json:"name"`
	Author         string                            `json:"author"`
	Usage          string                            `json:"usage"`
	Description    string                            `json:"description"`
	Cmd            map[string]*command.StringCommand `json:"cmd"`
	Actions        []string                          `json:"actions"`
	Hooks          []*PluginHooks                    `json:"hooks"`
	Supports       []string                          `json:"supports"`
	StateTypes     []string                          `json:"state_types"`
	SupportedTypes []*PluginType                     `json:"supported_types"`
	Commands       map[string]*PluginCommand         `json:"commands"`

	Dir      string          `json:"-"`
	CacheDir string          `json:"-"`
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
	ActionLock
	ActionState
)

func (p *Plugin) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.Name, validation.Required),
		validation.Field(&p.Cmd, validation.Required, validation.Map(validation.Key("default", validation.Required)).AllowExtraKeys()),
		validation.Field(&p.Actions, validation.Required),
		validation.Field(&p.Hooks),
		validation.Field(&p.Commands),
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

func (p *Plugin) Prepare(ctx context.Context, log logger.Logger, env, projectID, projectName, projectDir, hostAddr string, props map[string]interface{}, yamlPrefix string, yamlData []byte) error {
	runCommand, ok := p.Cmd[runtime.GOOS]
	if !ok {
		runCommand = p.Cmd["default"]
	}

	cmd := runCommand.ExecCmdAsUser()

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("OUTBLOCKS_BIN=%s", os.Args[0]),
		fmt.Sprintf("OUTBLOCKS_PLUGIN_DIR=%s", p.Dir),
		fmt.Sprintf("OUTBLOCKS_PLUGIN_PROJECT_CACHE_DIR=%s", p.CacheDir),
		fmt.Sprintf("OUTBLOCKS_ENV=%s", env),
		fmt.Sprintf("OUTBLOCKS_PROJECT_NAME=%s", projectName),
		fmt.Sprintf("OUTBLOCKS_PROJECT_ID=%s", projectID),
		fmt.Sprintf("OUTBLOCKS_PROJECT_DIR=%s", projectDir),
	)

	var err error

	p.client, err = client.NewClient(log, p.Name, env, cmd, hostAddr, props, client.YAMLContext{
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

func (p *Plugin) CommandArgs(cmd string) map[string]interface{} {
	c := p.Commands[cmd]
	if c == nil {
		return nil
	}

	flags := make(map[string]interface{})

	for _, flag := range c.Flags {
		flags[flag.Name] = flag.Val()
	}

	return flags
}

func ComputePluginID(name string) string {
	return fmt.Sprintf("plugin_%s", name)
}

func (p *Plugin) ID() string {
	return ComputePluginID(p.Name)
}

package plugins

import (
	"reflect"

	"github.com/blang/semver/v4"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/internal/version"
	"github.com/outblocks/outblocks-cli/pkg/lockfile"
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
	SupportedTypes []*PluginType  `json:"supported_types"`

	Path     string `json:"-"`
	yamlPath string
	version  *semver.Version
	source   string
	data     []byte
	actions  []Action
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
	)
}

func (p *Plugin) Locked() *lockfile.Plugin {
	return &lockfile.Plugin{
		Name:    p.Name,
		Version: &version.SemverVersion{Version: p.version},
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
		if t.Type == typ && (t.Match == nil || (t.Match.Deploy == dep || mapMatch(t.Match.Other, other))) {
			return true
		}
	}

	return false
}

type PluginType struct {
	Type  string       `json:"type"`
	Match *PluginMatch `json:"match"`
}

type PluginMatch struct {
	Deploy string                 `json:"deploy"`
	Other  map[string]interface{} `yaml:"-,remain"`
}

type PluginHooks struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

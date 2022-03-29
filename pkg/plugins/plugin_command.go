package plugins

import (
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/internal/util"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

type CommandValueType int

const (
	CommandValueTypeBool CommandValueType = iota + 1
	CommandValueTypeString
	CommandValueTypeInt
)

type CommandInputType int

const (
	CommandInputTypeAppStates CommandInputType = iota + 1
	CommandInputTypeDependencyStates
	CommandInputTypePluginState
)

var (
	CommandValueTypes = []string{"str", "string", "bool", "boolean", "int", "integer"}
	CommandInputTypes = []string{"app_states", "dependency_states", "plugin_state"}
)

type PluginCommand struct {
	Short string               `json:"short"`
	Long  string               `json:"long"`
	Input []string             `json:"input"`
	Flags []*PluginCommandFlag `json:"flags"`

	inputTypes []CommandInputType
}

func (p *PluginCommand) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.Input, validation.Each(validation.In(util.InterfaceSlice(CommandInputTypes)...))),
		validation.Field(&p.Flags),
	)
}

func (p *PluginCommand) Proto(args []string) *apiv1.CommandArgs {
	flags := make(map[string]interface{})

	for _, v := range p.Flags {
		flags[v.Name] = v.Val()
	}

	return &apiv1.CommandArgs{
		Positional: args,
		Flags:      plugin_util.MustNewStruct(flags),
	}
}

func (p *PluginCommand) InputTypes() []CommandInputType {
	if p.inputTypes != nil {
		return p.inputTypes
	}

	p.inputTypes = make([]CommandInputType, len(p.Input))

	for i, v := range p.Input {
		switch v {
		case "app_states":
			p.inputTypes[i] = CommandInputTypeAppStates
		case "dependency_states":
			p.inputTypes[i] = CommandInputTypeDependencyStates
		case "plugin_state":
			p.inputTypes[i] = CommandInputTypePluginState
		default:
			panic(fmt.Sprintf("unknown input type: %s", v))
		}
	}

	return p.inputTypes
}

type PluginCommandFlag struct {
	Name     string      `json:"name"`
	Short    string      `json:"short"`
	Usage    string      `json:"usage"`
	Type     string      `json:"type"`
	Default  interface{} `json:"default"`
	Required bool        `json:"required"`
	Value    interface{} `json:"-"`

	typ CommandValueType
}

func (p *PluginCommandFlag) Val() interface{} {
	if p.Value == nil {
		return nil
	}

	switch p.ValueType() {
	case CommandValueTypeBool:
		return *(p.Value.(*bool))
	case CommandValueTypeString:
		return *(p.Value.(*string))
	case CommandValueTypeInt:
		return *(p.Value.(*int))
	}

	return nil
}

func (p *PluginCommandFlag) ValueType() CommandValueType {
	if p.typ != 0 {
		return p.typ
	}

	switch p.Type {
	case "bool", "boolean":
		p.typ = CommandValueTypeBool
	case "int", "integer":
		p.typ = CommandValueTypeInt
	case "str", "string":
		p.typ = CommandValueTypeString
	default:
		panic(fmt.Sprintf("unknown command value type: %s", p.Type))
	}

	return p.typ
}

func (p *PluginCommandFlag) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.Name, validation.Required),
		validation.Field(&p.Usage, validation.Required),
		validation.Field(&p.Type, validation.In(util.InterfaceSlice(CommandValueTypes)...)),
	)
}

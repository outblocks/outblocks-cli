package plugins

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/internal/util"
)

const (
	CommandTypeBool   = "bool"
	CommandTypeString = "string"
	CommandTypeInt    = "int"
)

var (
	CommandTypes = []string{CommandTypeBool, CommandTypeString, CommandTypeInt}
)

type PluginCommand struct {
	Args []*PluginCommandArg `json:"args"`
}

func (p *PluginCommand) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.Args),
	)
}

type PluginCommandArg struct {
	Name    string      `json:"name"`
	Usage   string      `json:"usage"`
	Type    string      `json:"type"`
	Default interface{} `json:"default"`
	Value   interface{} `json:"-"`
}

func (p *PluginCommandArg) Val() interface{} {
	if p.Value == nil {
		return nil
	}

	switch p.Type {
	case CommandTypeBool:
		return *(p.Value.(*bool))
	case CommandTypeString:
		return *(p.Value.(*string))
	case CommandTypeInt:
		return *(p.Value.(*int))
	}

	return nil
}

func (p *PluginCommandArg) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.Name, validation.Required),
		validation.Field(&p.Usage, validation.Required),
		validation.Field(&p.Type, validation.In(util.InterfaceSlice(CommandTypes)...)),
	)
}

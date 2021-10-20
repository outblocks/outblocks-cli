package config

import (
	"github.com/goccy/go-yaml"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

type YAMLEvaluator struct {
	*plugin_util.BaseVarEvaluator
}

func NewYAMLEvaluator(vars map[string]interface{}) *YAMLEvaluator {
	return &YAMLEvaluator{
		BaseVarEvaluator: plugin_util.NewBaseVarEvaluator(vars).
			WithEncoder(yamlVarEncoder).
			WithIgnoreComments(true).
			WithVarChar('$'),
	}
}

func (e *YAMLEvaluator) Expand(input []byte) ([]byte, error) {
	format, _, err := e.ExpandRaw(input)
	return format, err
}

func yamlVarEncoder(val interface{}) ([]byte, error) {
	valOut, err := yaml.MarshalWithOptions(val, yaml.Flow(true))
	if err != nil {
		return nil, err
	}

	return valOut[:len(valOut)-1], nil
}

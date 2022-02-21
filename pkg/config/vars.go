package config

import (
	"bytes"
	"encoding/json"
	"regexp"

	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

var quoteValues = regexp.MustCompile(`(\s*-\s+|\S:\s+)(\$\{var\.[^}]+})`)

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
	input = quoteValues.ReplaceAll(input, []byte(`$1"$2"`))
	format, _, err := e.ExpandRaw(input)

	return format, err
}

func yamlVarEncoder(val interface{}) ([]byte, error) {
	valOut, err := json.Marshal(val)

	if err != nil {
		return nil, err
	}

	valOut = bytes.TrimPrefix(valOut, []byte{'"'})
	valOut = bytes.TrimSuffix(valOut, []byte{'"'})

	return valOut, nil
}

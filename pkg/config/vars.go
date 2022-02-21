package config

import (
	"encoding/json"
	"regexp"

	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

var (
	checkQuotePrefix = regexp.MustCompile(`(\s*-\s+|\S:\s+)$`)
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

func yamlVarEncoder(c *plugin_util.VarContext, val interface{}) ([]byte, error) {
	valOut, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}

	if valOut[0] == '"' && valOut[len(valOut)-1] == '"' {
		switch valOut[1] {
		case '*', '&', '[', '{', '}', ']', ',', '!', '|', '>', '%', '\'', '"':
			if len(valOut) > 2 && checkQuotePrefix.Match(c.Line[:c.TokenStart]) {
				return valOut, nil
			}
		}

		valOut = valOut[1 : len(valOut)-1]
	}

	return valOut, nil
}

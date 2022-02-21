package util

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ansel1/merry/v2"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

var escapePercent = regexp.MustCompile(`%([^{]|$)`)

type VarEvaluator struct {
	*plugin_util.BaseVarEvaluator
}

func NewVarEvaluator(vars map[string]interface{}) *VarEvaluator {
	return &VarEvaluator{
		BaseVarEvaluator: plugin_util.NewBaseVarEvaluator(vars).
			WithEncoder(varEncoder).
			WithVarChar('%').
			WithIgnoreInvalid(true),
	}
}

func varEncoder(c *plugin_util.VarContext, input interface{}) ([]byte, error) {
	switch input.(type) {
	case string:
		return []byte("%s"), nil
	case int:
		return []byte("%d"), nil
	}

	return nil, merry.New("unknown input type")
}

func (e *VarEvaluator) Expand(input string) (string, error) {
	input = escapePercent.ReplaceAllString(input, "%$0")

	format, params, err := e.ExpandRaw([]byte(input))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(strings.ReplaceAll(string(format), "%{", "%%{"), params...), nil
}

func (e *VarEvaluator) ExpandStringMap(input map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(input))

	var err error

	for k, v := range input {
		out[k], err = e.Expand(v)
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

func MapLookupPath(m map[string]interface{}, keys ...string) map[string]interface{} {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			return nil
		}

		m, ok = v.(map[string]interface{})
		if !ok {
			return nil
		}
	}

	return m
}

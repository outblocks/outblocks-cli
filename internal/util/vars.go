package util

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-plugin-go/util"
)

var escapePercent = regexp.MustCompile(`%([^{]|$)`)

type VarEvaluator struct {
	*util.BaseVarEvaluator
}

func NewVarEvaluator(vars map[string]interface{}) *VarEvaluator {
	return &VarEvaluator{
		BaseVarEvaluator: util.NewBaseVarEvaluator(vars).
			WithEncoder(varEncoder).
			WithVarChar('%').
			WithIgnoreInvalid(true),
	}
}

func varEncoder(input interface{}) ([]byte, error) {
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

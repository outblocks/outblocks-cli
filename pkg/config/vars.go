package config

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

type YAMLEvaluator struct {
	vars map[string]interface{}
}

func NewYAMLEvaluator(vars map[string]interface{}) *YAMLEvaluator {
	return &YAMLEvaluator{
		vars: vars,
	}
}

func (e *YAMLEvaluator) getKey(key string) (interface{}, bool) {
	var ok bool

	vars := e.vars
	parts := strings.Split(key, ".")

	for _, part := range parts[:len(parts)-1] {
		vars, ok = vars[part].(map[string]interface{})
		if !ok {
			return nil, false
		}
	}

	v, ok := vars[parts[len(parts)-1]]

	return v, ok
}

func (e *YAMLEvaluator) encode(val interface{}) ([]byte, error) {
	valOut, err := yaml.MarshalWithOptions(val, yaml.Flow(true))
	if err != nil {
		return nil, err
	}

	return valOut[:len(valOut)-1], nil
}

func (e *YAMLEvaluator) Expand(input []byte) ([]byte, error) {
	var out []byte

	done := 0
	line := 0
	lineStart := 0
	tokenStart := -1
	ll := len(input)

	var token string

	for i := 0; i < ll; i++ {
		switch {
		case input[i] == '\n':
			line++

			lineStart = i + 1

			continue

		case tokenStart == -1:
			if i+1 < ll && input[i] == '$' && input[i+1] == '{' {
				i++
				tokenStart = i
			}

		case input[i] == '}':
			token = string(input[tokenStart+1 : i])
			if token == "" {
				return nil, fmt.Errorf("[%d:%d] empty expansion found", line+1, tokenStart-lineStart)
			}

			out = append(out, input[done:tokenStart-1]...)

			val, ok := e.getKey(token)
			if !ok {
				return nil, fmt.Errorf("[%d:%d] expansion value '%s' could not be evaluated", line+1, tokenStart-lineStart, token)
			}

			valOut, err := e.encode(val)
			if err != nil {
				return nil, fmt.Errorf("[%d:%d] expansion value '%s' could not be marshaled\nerror: %w",
					line+1, tokenStart-lineStart, token, err)
			}

			out = append(out, valOut...)

			done = i + 1
			tokenStart = -1
		}
	}

	out = append(out, input[done:]...)

	return out, nil
}

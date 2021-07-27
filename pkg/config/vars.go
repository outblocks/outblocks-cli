package config

import (
	"bytes"
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
	tokenStart := -1

	var token string

	in := bytes.Split(input, []byte{'\n'})
	out := make([][]byte, len(in))

	for l, line := range in {
		ll := len(line)
		done := 0

		lineTrimmed := bytes.TrimSpace(line)
		if len(lineTrimmed) > 0 && lineTrimmed[0] == '#' {
			out[l] = line

			continue
		}

		for c := range line {
			switch {
			case tokenStart == -1:
				if c+1 < ll && line[c] == '$' && line[c+1] == '{' {
					c++
					tokenStart = c
				}

			case line[c] == '}':
				token = string(line[tokenStart+1 : c])
				if token == "" {
					return nil, fmt.Errorf("[%d:%d] empty expansion found", l+1, tokenStart)
				}

				out[l] = append(out[l], line[done:tokenStart-1]...)

				val, ok := e.getKey(token)
				if !ok {
					return nil, fmt.Errorf("[%d:%d] expansion value '%s' could not be evaluated", l+1, tokenStart, token)
				}

				valOut, err := e.encode(val)
				if err != nil {
					return nil, fmt.Errorf("[%d:%d] expansion value '%s' could not be marshaled\nerror: %w",
						l+1, tokenStart, token, err)
				}

				out[l] = append(out[l], valOut...)

				done = c + 1
				tokenStart = -1
			}
		}

		out[l] = append(out[l], line[done:]...)
	}

	return bytes.Join(out, []byte{'\n'}), nil
}

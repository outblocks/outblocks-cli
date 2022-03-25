package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/ansel1/merry/v2"
	"github.com/enescakir/emoji"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/printer"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func isPathInMap(path []string, m map[string]bool) bool {
	for i := len(path); i >= 0; i-- {
		if m[strings.Join(path[:i], ".")] {
			return true
		}
	}

	return false
}

func expandYAMLString(n *ast.StringNode, file string, vars map[string]interface{}, essentialKeys map[string]bool, path []string) (ast.Node, error) {
	tknCount := strings.Count(n.Value, "${")
	if tknCount == 0 {
		return n, nil
	}

	complexType := false

	output, _, err := plugin_util.NewBaseVarEvaluator(vars).
		WithEncoder(func(c *plugin_util.VarContext, val interface{}) ([]byte, error) {
			t := reflect.TypeOf(val)

			switch {
			case t.Kind() == reflect.Slice || t.Kind() == reflect.Map:
				if len(c.Input) != (c.TokenColumnEnd - c.TokenColumnStart + 1) {
					return nil, fmt.Errorf("to substitute non-primitive value, it cannot be a part of a string and needs to be unquoted")
				}

				complexType = true

			case t.Kind() == reflect.String:
				return []byte(val.(string)), nil
			}

			return json.Marshal(val)
		}).
		WithSkipRowColumnInfo(true).
		WithVarChar('$').ExpandRaw([]byte(n.Value))
	if err != nil {
		if essentialKeys == nil || isPathInMap(path, essentialKeys) {
			var pp printer.Printer
			annotate := pp.PrintErrorToken(n.GetToken(), yaml.DefaultColorize())

			return nil, merry.Errorf("\nfile: %s\n%s\n\n%s  value expansion failed: %w", file, annotate, emoji.Warning, err)
		}

		return n, nil
	}

	if !complexType {
		output = []byte(fmt.Sprintf("%q", string(output)))
	}

	tok, err := parser.ParseBytes(output, 0)
	if err != nil {
		var pp printer.Printer
		annotate := pp.PrintErrorToken(n.GetToken(), yaml.DefaultColorize())

		return nil, merry.Errorf("\nfile: %s\n%s\n\n%s  error parsing resulting yaml: %w", file, annotate, emoji.Warning, err)
	}

	return tok.Docs[0].Body, nil
}

func traverseYAMLMapping(node ast.Node, file string, vars map[string]interface{}, essentialKeys map[string]bool, path []string) (ast.Node, error) {
	switch n := node.(type) {
	case *ast.StringNode:
		return expandYAMLString(n, file, vars, essentialKeys, path)

	case *ast.MappingValueNode:
		newVal, err := traverseYAMLMapping(n.Value, file, vars, essentialKeys, append(append([]string(nil), path...), n.Key.String()))
		if err != nil {
			return nil, err
		}

		n.Value = newVal

	case *ast.MappingNode:
		for _, v := range n.Values {
			keyPath := append(append([]string(nil), path...), v.Key.String())

			newVal, err := traverseYAMLMapping(v.Value, file, vars, essentialKeys, keyPath)
			if err != nil {
				return nil, err
			}

			v.Value = newVal
		}

	case *ast.SequenceNode:
		for i, v := range n.Values {
			newVal, err := traverseYAMLMapping(v, file, vars, essentialKeys, path)
			if err != nil {
				return nil, err
			}

			n.Values[i] = newVal
		}
	}

	return node, nil
}

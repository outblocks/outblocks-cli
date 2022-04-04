package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/23doors/go-yaml"
	"github.com/23doors/go-yaml/ast"
	"github.com/23doors/go-yaml/parser"
	"github.com/23doors/go-yaml/printer"
	"github.com/ansel1/merry/v2"
	"github.com/enescakir/emoji"
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

func expandYAMLString(n *ast.StringNode, file, env string, vals map[string]interface{}, essentialKeys map[string]bool, path []string) (ast.Node, error) {
	tknCount := strings.Count(n.Value, "${")
	if tknCount == 0 {
		return n, nil
	}

	var (
		t         reflect.Type
		fullValue bool
	)

	output, _, err := plugin_util.NewBaseVarEvaluator(vals).
		WithEncoder(func(c *plugin_util.VarContext, val interface{}) ([]byte, error) {
			t = reflect.TypeOf(val)
			fullValue = len(c.Input) == (c.TokenColumnEnd - c.TokenColumnStart + 1)

			switch {
			case t.Kind() == reflect.Slice || t.Kind() == reflect.Map:
				if !fullValue {
					return nil, fmt.Errorf("to substitute non-primitive value, it cannot be a part of a string")
				}

			case t.Kind() == reflect.String:
				return []byte(val.(string)), nil
			}

			return json.Marshal(val)
		}).
		WithKeyGetter(func(c *plugin_util.VarContext, vars map[string]interface{}) (val interface{}, err error) {
			if strings.HasPrefix(c.Token, "var.") {
				if val, ok := os.LookupEnv(fmt.Sprintf("OUTBLOCKS_VALUE_%s_%s", strings.ToUpper(env), c.Token[4:])); ok {
					return val, nil
				}

				if val, ok := os.LookupEnv(fmt.Sprintf("OUTBLOCKS_VALUE_%s", c.Token[4:])); ok {
					return val, nil
				}
			}

			return plugin_util.DefaultVarKeyGetter(c, vars)
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

	if !fullValue || t.Kind() == reflect.String {
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

func traverseYAMLMapping(node ast.Node, file, env string, vals map[string]interface{}, essentialKeys map[string]bool, path []string) (ast.Node, error) {
	switch n := node.(type) {
	case *ast.StringNode:
		return expandYAMLString(n, file, env, vals, essentialKeys, path)

	case *ast.MappingValueNode:
		newVal, err := traverseYAMLMapping(n.Value, file, env, vals, essentialKeys, append(append([]string(nil), path...), n.Key.String()))
		if err != nil {
			return nil, err
		}

		n.Value = newVal

	case *ast.MappingNode:
		for _, v := range n.Values {
			keyPath := append(append([]string(nil), path...), v.Key.String())

			newVal, err := traverseYAMLMapping(v.Value, file, env, vals, essentialKeys, keyPath)
			if err != nil {
				return nil, err
			}

			v.Value = newVal
		}

	case *ast.SequenceNode:
		for i, v := range n.Values {
			newVal, err := traverseYAMLMapping(v, file, env, vals, essentialKeys, path)
			if err != nil {
				return nil, err
			}

			n.Values[i] = newVal
		}
	}

	return node, nil
}

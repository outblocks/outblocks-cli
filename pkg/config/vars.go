package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/ansel1/merry/v2"
	"github.com/enescakir/emoji"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/printer"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

var (
	checkQuotePrefix = regexp.MustCompile(`(\s*-\s+|\S:\s+)$`)
)

func isPathInMap(path []string, m map[string]bool) bool {
	for i := len(path); i >= 0; i-- {
		if m[strings.Join(path[:i], ".")] {
			return true
		}
	}

	return false
}

func traverseYAMLMapping(file string, vars map[string]interface{}, m *ast.MappingNode, essentialKeys map[string]bool, path []string) error {
	for _, mv := range m.Values {
		key := mv.Key.String()
		curPath := path

		curPath = append(curPath, key)

		switch val := mv.Value.(type) {
		case *ast.MappingNode:
			err := traverseYAMLMapping(file, vars, val, essentialKeys, curPath)
			if err != nil {
				return err
			}
		case *ast.StringNode:
			// if indicator or multiple values here - that's an error
			tknCount := strings.Count(val.String(), "${")
			if tknCount == 0 {
				continue
			}

			output, _, err := plugin_util.NewBaseVarEvaluator(vars).
				WithEncoder(func(c *plugin_util.VarContext, val interface{}) ([]byte, error) {
					t := reflect.TypeOf(val)

					if t.Kind() == reflect.Slice || t.Kind() == reflect.Map {
						if len(c.Input) != (c.TokenColumnEnd - c.TokenColumnStart + 1) {
							return nil, fmt.Errorf("to substitute non-primitive value, it cannot be a part of a string and needs to be unquoted")
						}
					}

					valOut, err := json.Marshal(val)
					if err != nil {
						return nil, err
					}

					if valOut[0] == '"' && valOut[len(valOut)-1] == '"' {
						switch valOut[1] {
						case '*', '&', '[', '{', '}', ']', ',', '!', '|', '>', '%', '\'', '"':
							if len(valOut) > 2 && checkQuotePrefix.Match(c.Line[:c.TokenColumnStart]) {
								return valOut, nil
							}
						}

						valOut = valOut[1 : len(valOut)-1]
					}

					return valOut, nil
				}).
				WithSkipRowColumnInfo(true).
				WithVarChar('$').ExpandRaw([]byte(val.String()))
			if err != nil {
				if essentialKeys == nil || isPathInMap(curPath, essentialKeys) {
					var pp printer.Printer
					annotate := pp.PrintErrorToken(val.GetToken(), yaml.DefaultColorize())

					return merry.Errorf("\nfile: %s\n%s\n\n%s  value expansion failed: %w", file, annotate, emoji.Warning, err)
				}

				continue
			}

			tok, err := parser.ParseBytes(output, 0)
			if err != nil {
				var pp printer.Printer
				annotate := pp.PrintErrorToken(val.GetToken(), yaml.DefaultColorize())

				return merry.Errorf("\nfile: %s\n%s\n\n%s  error parsing resulting yaml: %w", file, annotate, emoji.Warning, err)
			}

			mv.Value = tok.Docs[0].Body
		}
	}

	return nil
}

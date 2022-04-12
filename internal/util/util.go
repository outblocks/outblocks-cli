package util

import (
	"fmt"
	"os"
	"reflect"
	"regexp"

	"github.com/23doors/go-yaml"
	"github.com/23doors/go-yaml/ast"
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"golang.org/x/term"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func InterfaceSlice(slice interface{}) []interface{} {
	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic("InterfaceSlice() given a non-slice type")
	}

	if s.IsNil() {
		return nil
	}

	ret := make([]interface{}, s.Len())

	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}

	return ret
}

func RegexValidator(regex *regexp.Regexp, msg string) func(interface{}) error {
	return func(val interface{}) error {
		// since we are validating an Input, the assertion will always succeed
		if str, ok := val.(string); !ok || !regex.MatchString(str) {
			return merry.New(msg)
		}

		return nil
	}
}

func CopyMapStringString(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))

	for k, v := range m {
		out[k] = v
	}

	return out
}

func FlattenEnvMap(m map[string]string) []string {
	out := make([]string, 0, len(m))

	for k, v := range m {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}

	return out
}

func StringArrayToSet(in []string) map[string]bool {
	outMap := make(map[string]bool, len(in))

	for _, i := range in {
		outMap[i] = true
	}

	return outMap
}

func IsTerminal() bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}

	if os.Getenv("CI") == "true" || os.Getenv("TERM") == "dumb" {
		return false
	}

	return true
}

var newline = regexp.MustCompile(`\n`)

func IndentString(s, indent string) string {
	if s == "" {
		return s
	}

	return fmt.Sprintf("%s%s", indent, newline.ReplaceAllString(s, fmt.Sprintf("\n%s", indent)))
}

func Title(s string) string {
	return cases.Title(language.Und, cases.NoLower).String(s)
}

func YAMLNodeDecode(n ast.Node, out interface{}) error {
	err := yaml.NodeToValue(n, out,
		yaml.Validator(validator.DefaultValidator()),
	)

	return err
}

func YAMLUnmarshal(in []byte, out interface{}) error {
	err := yaml.UnmarshalWithOptions(in, out,
		yaml.Validator(validator.DefaultValidator()),
		yaml.UseJSONUnmarshaler(),
	)

	return err
}

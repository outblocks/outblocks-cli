package util

import (
	"fmt"
	"os"
	"reflect"
	"regexp"

	"github.com/ansel1/merry/v2"
	"golang.org/x/term"
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

func IsInteractive() bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}

	if os.Getenv("CI") == "true" {
		return false
	}

	return true
}

func IsTermDumb() bool {
	if os.Getenv("CI") == "true" || os.Getenv("TERM") == "dumb" {
		return true
	}

	return false
}

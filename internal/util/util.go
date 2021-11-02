package util

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
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
			return errors.New(msg)
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

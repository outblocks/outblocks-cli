package util

import (
	"errors"
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

func MergeMaps(a ...map[string]interface{}) map[string]interface{} {
	if len(a) == 0 {
		return nil
	}

	out := make(map[string]interface{}, len(a[0]))
	for k, v := range a[0] {
		out[k] = v
	}

	for _, m := range a[1:] {
		for k, v := range m {
			if v, ok := v.(map[string]interface{}); ok {
				if bv, ok := out[k]; ok {
					if bv, ok := bv.(map[string]interface{}); ok {
						out[k] = MergeMaps(bv, v)
						continue
					}
				}
			}

			out[k] = v
		}
	}

	return out
}

func MergeStringMaps(a ...map[string]string) map[string]string {
	if len(a) == 0 {
		return nil
	}

	out := make(map[string]string, len(a[0]))

	for _, m := range a {
		for k, v := range m {
			out[k] = v
		}
	}

	return out
}

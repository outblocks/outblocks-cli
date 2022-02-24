package statediff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/util"
)

type MapDiff struct {
	nested   map[interface{}]*MapDiff
	update   map[interface{}]interface{}
	delete   map[interface{}]struct{}
	maxLevel int
	level    int
}

func NewMapDiff(m1, m2 interface{}, maxLevel int) (*MapDiff, error) {
	return newMapDiff(m1, m2, maxLevel, 0)
}

func newMapDiff(m1, m2 interface{}, maxLevel, level int) (*MapDiff, error) {
	m1i := convertMap(m1)
	m2i := convertMap(m2)

	ret := &MapDiff{
		nested:   make(map[interface{}]*MapDiff),
		update:   make(map[interface{}]interface{}),
		delete:   make(map[interface{}]struct{}),
		maxLevel: maxLevel,
		level:    level,
	}

	for k, v := range m1i {
		v2, ok := m2i[k]

		delete(m2i, k)

		if !ok {
			ret.delete[k] = struct{}{}
			continue
		}

		if level+1 < maxLevel && reflect.TypeOf(v).Kind() == reflect.Map && reflect.TypeOf(v2).Kind() == reflect.Map {
			mdiff, err := newMapDiff(v, v2, maxLevel, level+1)
			if err != nil {
				return nil, err
			}

			if mdiff.IsEmpty() {
				continue
			}

			ret.nested[k] = mdiff

			continue
		}

		jd1, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		jd2, err := json.Marshal(v2)
		if err != nil {
			return nil, err
		}

		if bytes.Equal(jd1, jd2) {
			continue
		}

		ret.update[k] = v2
	}

	for k, v := range m2i {
		ret.update[k] = v
	}

	return ret, nil
}

func (d *MapDiff) Apply(m interface{}) {
	rv := reflect.ValueOf(m)

	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Map {
		panic("invalid object type passed to map")
	}

	for k := range d.delete {
		rv.SetMapIndex(reflect.ValueOf(k), reflect.Value{})
	}

	for k, v := range d.update {
		rv.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
	}

	for k, v := range d.nested {
		i := rv.MapIndex(reflect.ValueOf(k)).Interface()
		if i == nil {
			i = make(map[string]interface{})
		}

		v.Apply(i)
		rv.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(i))
	}
}

func (d *MapDiff) IsEmpty() bool {
	return len(d.delete) == 0 && len(d.update) == 0 && len(d.nested) == 0
}

func (d *MapDiff) String() string {
	ret := ""

	for k := range d.delete {
		ret += fmt.Sprintf("x delete: %#v\n", k)
	}

	for k, v := range d.update {
		if md, ok := v.(*MapDiff); ok {
			ret += fmt.Sprintf("~ update: %#v\n%s\n", k, util.IndentString(md.String(), "  "))

			continue
		}

		js, _ := json.MarshalIndent(v, "  ", "  ")
		ret += fmt.Sprintf("~ update: %#v\n  to: %s\n", k, string(js))
	}

	for k, md := range d.nested {
		ret += fmt.Sprintf("~ nested: %#v\n%s\n", k, util.IndentString(md.String(), "  "))
	}

	return strings.TrimRight(ret, "\n")
}

func convertMap(m interface{}) map[interface{}]interface{} {
	rv := reflect.ValueOf(m)
	ret := make(map[interface{}]interface{}, rv.Len())

	iter := rv.MapRange()

	for iter.Next() {
		ret[iter.Key().Interface()] = iter.Value().Interface()
	}

	return ret
}

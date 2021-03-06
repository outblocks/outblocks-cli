package validator

import (
	"reflect"

	"github.com/23doors/go-yaml"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

func DefaultValidator() yaml.StructValidator {
	return structValidator{}
}

type structValidator struct{}

func (v structValidator) Struct(i interface{}) error {
	if v, ok := i.(validation.Validatable); ok {
		return v.Validate()
	}

	val := reflect.ValueOf(i)
	vp := reflect.New(val.Type())
	vp.Elem().Set(val)

	i = vp.Interface()

	if v, ok := i.(validation.Validatable); ok {
		return v.Validate()
	}

	return nil
}

package jsonschemaform

import (
	"bytes"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func Validate(schema []byte, i interface{}) error {
	js := jsonschema.NewCompiler()
	js.Draft = jsonschema.Draft7

	err := js.AddResource("schema.json", bytes.NewReader(schema))
	if err != nil {
		return err
	}

	sch, err := js.Compile("schema.json")
	if err != nil {
		return err
	}

	return sch.Validate(i)
}

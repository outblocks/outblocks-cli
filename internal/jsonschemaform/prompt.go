package jsonschemaform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/pterm/pterm"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

func schema(sch *jsonschema.Schema) *jsonschema.Schema {
	for sch.Ref != nil {
		sch = sch.Ref
	}

	return sch
}

func getSchemaType(sch *jsonschema.Schema) string {
	if len(sch.Types) == 1 {
		return sch.Types[0]
	}

	return ""
}

func toStringMap(list []string) map[string]bool {
	m := make(map[string]bool, len(list))

	for _, k := range list {
		m[k] = true
	}

	return m
}

func checkValidMulti(sch []*jsonschema.Schema, prop string, value interface{}) []*jsonschema.Schema {
	var valid []*jsonschema.Schema

	for _, s := range sch {
		dprop, ok := s.Properties.Get(prop)
		if !ok {
			continue
		}

		if dprop.(*jsonschema.Schema).Validate(value) == nil {
			valid = append(valid, s)
		}
	}

	return valid
}

func processObjectAnyOneOf(level int, prefix, key string, sch []*jsonschema.Schema, ret map[string]interface{}) error {
	var opts []string

	optsMap := make(map[string]*jsonschema.Schema)

	for i, s := range sch {
		title := fmt.Sprintf("Option %d", i+1)
		if s.Title != "" {
			title = s.Title
		}

		opts = append(opts, title)
		optsMap[title] = s
	}

	var opt string

	err := survey.AskOne(&survey.Select{
		Message: "Choose:",
		Options: opts,
	}, &opt)
	if err != nil {
		return err
	}

	vals, err := processObject(level, prefix, key, optsMap[opt], ret)
	if err != nil {
		return err
	}

	for k, v := range vals {
		ret[k] = v
	}

	return nil
}

func processObjectDependencies(level int, prefix, key string, sch *jsonschema.Schema, ret map[string]interface{}) (map[string]interface{}, error) {
	for dname, dep := range sch.Dependencies {
		if _, ok := sch.Properties.Get(dname); !ok {
			continue
		}

		dep, ok := dep.(*jsonschema.Schema)
		if !ok {
			continue
		}

		dep = schema(dep)

		if len(dep.OneOf) > 0 {
			valid := checkValidMulti(dep.OneOf, dname, ret[dname])

			if len(valid) == 1 {
				vcopy := *valid[0]
				props := *vcopy.Properties
				props.Delete(dname)
				vcopy.Properties = &props

				vals, err := processObject(level, prefix, key, &vcopy, ret)
				if err != nil {
					return nil, err
				}

				for k, v := range vals {
					ret[k] = v
				}
			}

			continue
		}

		depReq := toStringMap(dep.Required)

		if ret[dname] != nil {
			for _, propKey := range dep.Properties.Keys() {
				prop, _ := dep.Properties.Get(propKey)

				val, err := process(level, prefix, propKey, prop.(*jsonschema.Schema), depReq[propKey])
				if err != nil {
					return nil, err
				}

				ret[propKey] = val
			}
		}
	}

	return ret, nil
}

func processObjectAdditionalProperties(level int, prefix, key string, sch *jsonschema.Schema, ret map[string]interface{}) (map[string]interface{}, error) {
	props, ok := sch.AdditionalProperties.(*jsonschema.Schema)
	if !ok {
		return ret, nil
	}

	props = schema(props)

	keyTitle := sch.Title
	if keyTitle == "" {
		keyTitle = key
	}

	keyTitle = fmt.Sprintf("%s%s", prefix, keyTitle)

	prefix = "Additional property"
	if props.Title != "" {
		prefix = fmt.Sprintf("%s property", props.Title)
	}

	for {
		var confirm bool

		err := survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("%s: Add additional property?", keyTitle),
			Default: true,
		}, &confirm)
		if err != nil {
			return nil, err
		}

		if !confirm {
			break
		}

		var key string

		err = survey.AskOne(
			&survey.Input{
				Message: fmt.Sprintf("%s key", prefix),
			}, &key, survey.WithValidator(func(ans interface{}) error {
				if v, ok := ans.(string); ok {
					if _, ok := ret[v]; ok {
						return fmt.Errorf("key '%s' already exists", v)
					}
				}

				return nil
			}))
		if err != nil {
			return nil, err
		}

		if key == "" {
			return ret, nil
		}

		val, err := process(level, fmt.Sprintf("%s value: ", prefix), key, props, false)
		if err != nil {
			if err == terminal.InterruptErr {
				return ret, nil
			}

			return nil, err
		}

		ret[key] = val
	}

	return ret, nil
}

func processObject(level int, prefix, key string, sch *jsonschema.Schema, values map[string]interface{}) (map[string]interface{}, error) {
	sch = schema(sch)
	req := toStringMap(sch.Required)

	var ret map[string]interface{}

	if values != nil {
		ret = values
	} else {
		ret = make(map[string]interface{})
	}

	for _, k := range sch.Properties.Keys() {
		val, _ := sch.Properties.Get(k)

		v, err := process(level, prefix, k, val.(*jsonschema.Schema), req[k])
		if err != nil {
			return nil, err
		}

		if v != nil {
			ret[k] = v
		}
	}

	if len(sch.AnyOf) > 0 {
		err := processObjectAnyOneOf(level, prefix, key, sch.AnyOf, ret)
		if err != nil {
			return nil, err
		}
	}

	if len(sch.OneOf) > 0 {
		err := processObjectAnyOneOf(level, prefix, key, sch.OneOf, ret)
		if err != nil {
			return nil, err
		}
	}

	for _, of := range sch.AllOf {
		vals, err := processObject(level, prefix, key, of, ret)
		if err != nil {
			return nil, err
		}

		for k, v := range vals {
			ret[k] = v
		}
	}

	ret, err := processObjectDependencies(level, prefix, key, sch, ret)
	if err != nil {
		return nil, err
	}

	// Parse additional properties if they exist.
	ret, err = processObjectAdditionalProperties(level, prefix, key, sch, ret)

	return ret, err
}

func promptArrayStandard(level int, keyTitle, prefix, key string, arraySch, itemSch *jsonschema.Schema) ([]interface{}, error) {
	var ret []interface{}

	itemSch = schema(itemSch)

	if arraySch.UniqueItems && len(itemSch.Enum) != 0 {
		var typ string
		if len(itemSch.Types) == 1 {
			typ = itemSch.Types[0]
		}

		var arrStr []string

		selectOpts := make([]string, len(itemSch.Enum))
		for i, v := range itemSch.Enum {
			selectOpts[i] = fmt.Sprintf("%s", v)
		}

		var def interface{}

		if schDef, ok := arraySch.Default.([]interface{}); ok {
			defArr := make([]string, len(schDef))
			for i, v := range schDef {
				defArr[i] = fmt.Sprintf("%s", v)
			}

			def = defArr
		}

		err := survey.AskOne(&survey.MultiSelect{
			Message: addLevelPrefix(level, keyTitle),
			Default: def,
			Help:    arraySch.Description,
			Options: selectOpts,
		}, &arrStr)
		if err != nil {
			return nil, err
		}

		switch typ {
		case "integer":
			ret = make([]interface{}, len(arrStr))
			for i, v := range arrStr {
				ret[i], _ = strconv.Atoi(v)
			}

			return ret, nil
		case "number":
			ret = make([]interface{}, len(arrStr))
			for i, v := range arrStr {
				ret[i], _ = strconv.ParseFloat(v, 64)
			}

			return ret, nil
		case "string":
			ret = make([]interface{}, len(arrStr))
			for i, v := range arrStr {
				ret[i] = v
			}

			return ret, nil
		default:
			panic("unknown type")
		}
	}

	for {
		var confirm bool

		err := survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("%s: Add additional element to array?", keyTitle),
			Default: true,
		}, &confirm)
		if err != nil {
			return nil, err
		}

		if !confirm {
			break
		}

		val, err := process(level, fmt.Sprintf("%s: ", prefix), key, itemSch, false)
		if err != nil {
			return nil, err
		}

		ret = append(ret, val)
	}

	return ret, nil
}

func promptArrayFixed(level int, keyTitle, prefix, key string, arraySch *jsonschema.Schema, itemSch []*jsonschema.Schema) ([]interface{}, error) {
	var ret []interface{}

	for _, itm := range itemSch {
		itm = schema(itm)

		val, err := process(level, fmt.Sprintf("%s: ", prefix), key, itm, false)
		if err != nil {
			return nil, err
		}

		ret = append(ret, val)
	}

	if itm, ok := arraySch.AdditionalItems.(*jsonschema.Schema); ok {
		itm = schema(itm)

		for {
			var confirm bool

			err := survey.AskOne(&survey.Confirm{
				Message: fmt.Sprintf("%s: Add additional element to array?", keyTitle),
				Default: true,
			}, &confirm)
			if err != nil {
				return nil, err
			}

			if !confirm {
				break
			}

			val, err := process(level, prefix, key, itm, false)
			if err != nil {
				return nil, err
			}

			ret = append(ret, val)
		}
	}

	return ret, nil
}

func promptArray(level int, key string, sch *jsonschema.Schema) ([]interface{}, error) {
	sch = schema(sch)

	prefix := "Array value"
	if sch.Title != "" {
		prefix = sch.Title
	}

	keyTitle := sch.Title
	if keyTitle == "" {
		keyTitle = key
	}

	switch itemSch := sch.Items.(type) {
	case *jsonschema.Schema:
		// Standard array.
		return promptArrayStandard(level, keyTitle, prefix, key, sch, itemSch)

	case []*jsonschema.Schema:
		// Fixed items array.
		return promptArrayFixed(level, keyTitle, prefix, key, sch, itemSch)
	}

	return nil, nil
}

func validateValue(val interface{}, sch *jsonschema.Schema) error {
	err := sch.Validate(val)
	if e, ok := err.(*jsonschema.ValidationError); ok {
		return errors.New(e.Causes[0].Message)
	}

	return err
}

func surveyPrompt(sch *jsonschema.Schema, msg string, def interface{}) survey.Prompt {
	if len(sch.Enum) == 0 {
		return &survey.Input{
			Message: msg,
			Default: def.(string),
			Help:    sch.Description,
		}
	}

	selectOpts := make([]string, len(sch.Enum))
	for i, v := range sch.Enum {
		selectOpts[i] = fmt.Sprintf("%s", v)
	}

	return &survey.Select{
		Message: msg,
		Default: def,
		Help:    sch.Description,
		Options: selectOpts,
	}
}

func Prompt(level int, schema []byte) (interface{}, error) {
	js := jsonschema.NewCompiler()
	js.Draft = jsonschema.Draft7
	js.ExtractAnnotations = true

	err := js.AddResource("schema.json", bytes.NewReader(schema))
	if err != nil {
		return nil, err
	}

	sch, err := js.Compile("schema.json")
	if err != nil {
		return nil, err
	}

	return process(level, "", "root", sch, false)
}

func addLevelPrefix(level int, msg string) string {
	return fmt.Sprintf("%s %s", strings.Repeat("#", level+1), msg)
}

func process(level int, prefix, key string, sch *jsonschema.Schema, required bool) (interface{}, error) { // nolint:gocyclo
	sch = schema(sch)

	typ := getSchemaType(sch)
	if typ == "" && key == "root" {
		typ = "object"
	}

	keyTitle := sch.Title
	if keyTitle == "" {
		keyTitle = key
	}

	keyTitle = fmt.Sprintf("%s%s", prefix, keyTitle)

	var opts []survey.AskOpt

	if required {
		opts = append(opts, survey.WithValidator(survey.Required))
	}

	switch typ {
	case "object":
		for {
			if key != "root" {
				fmt.Println()
			}

			if sch.Title != "" {
				pterm.FgYellow.Println(addLevelPrefix(level, sch.Title))
			}

			if sch.Description != "" {
				pterm.FgGray.Println(addLevelPrefix(level, sch.Description))
			}

			val, err := processObject(level+1, prefix, key, sch, nil)
			if err != nil {
				return nil, err
			}

			err = sch.Validate(val)
			if err != nil {
				if e, ok := err.(*jsonschema.ValidationError); ok {
					pterm.FgRed.Printf("Object validation error: %s\n", e.Causes[0].Message)
					fmt.Println()

					continue
				}

				return nil, err
			}

			return val, nil
		}

	case "string":
		var (
			o   string
			def string
		)

		if sch.Default != nil {
			def = fmt.Sprintf("%s", sch.Default)
		}

		opts = append(opts, survey.WithValidator(func(ans interface{}) error {
			if val, ok := ans.(string); ok {
				return validateValue(val, sch)
			}

			return nil
		}))

		err := survey.AskOne(surveyPrompt(sch, keyTitle, def), &o, opts...)
		if err != nil {
			return nil, err
		}

		return o, nil

	case "number":
		var (
			o   string
			def string
		)

		if v, ok := sch.Default.(json.Number); ok {
			def = v.String()
		}

		opts = append(opts, survey.WithValidator(func(ans interface{}) error {
			if v, ok := ans.(string); ok {
				if v == "" {
					return nil
				}

				val, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return errors.New("expecting an number")
				}

				return validateValue(val, sch)
			}

			return nil
		}))

		err := survey.AskOne(surveyPrompt(sch, keyTitle, def), &o, opts...)
		if err != nil {
			return nil, err
		}

		v, _ := strconv.ParseFloat(o, 64)

		return v, nil

	case "integer":
		var (
			o   string
			def string
		)

		if v, ok := sch.Default.(json.Number); ok {
			def = v.String()
		}

		opts = append(opts, survey.WithValidator(func(ans interface{}) error {
			if v, ok := ans.(string); ok {
				if v == "" {
					return nil
				}

				val, err := strconv.Atoi(v)
				if err != nil {
					return errors.New("expecting an integer")
				}

				return validateValue(val, sch)
			}

			return nil
		}))

		err := survey.AskOne(surveyPrompt(sch, keyTitle, def), &o, opts...)
		if err != nil {
			return nil, err
		}

		v, _ := strconv.Atoi(o)

		return v, nil

	case "boolean":
		var (
			o   bool
			def bool
		)

		if v, ok := sch.Default.(bool); ok {
			def = v
		}

		opts = append(opts, survey.WithValidator(func(ans interface{}) error {
			if val, ok := ans.(bool); ok {
				return validateValue(val, sch)
			}

			return nil
		}))

		err := survey.AskOne(&survey.Confirm{
			Message: keyTitle,
			Default: def,
			Help:    sch.Description,
		}, &o, opts...)
		if err != nil {
			return nil, err
		}

		return o, nil

	case "array":
		if sch.Title != "" && !sch.UniqueItems {
			pterm.FgBlue.Println(addLevelPrefix(level, sch.Title))
		}

		if sch.Description != "" {
			pterm.FgGray.Println(addLevelPrefix(level, sch.Description))
		}

		for {
			val, err := promptArray(level+1, key, sch)
			if err != nil {
				return nil, err
			}

			err = sch.Validate(val)
			if err != nil {
				if e, ok := err.(*jsonschema.ValidationError); ok {
					pterm.FgRed.Printf("Array validation error: %s\n", e.Causes[0].Message)
					continue
				}

				return nil, err
			}

			return val, nil
		}

	case "null":
		pterm.FgWhite.Println(keyTitle)

		if sch.Description != "" {
			pterm.FgGray.Println(sch.Description)
		}

		return nil, nil
	}

	panic("unknown type")
}

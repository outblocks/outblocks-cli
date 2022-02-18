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
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/pterm/pterm"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

const (
	JSONTypeObject  = "object"
	JSONTypeNumber  = "number"
	JSONTypeInteger = "integer"
	JSONTypeString  = "string"
	JSONTypeBoolean = "boolean"
	JSONTypeArray   = "array"
	JSONTypeNull    = "null"
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

type Node struct {
	level    int
	prefix   string
	key      string
	required bool
	input    map[string]interface{}
}

func (n *Node) increaseLevel() *Node {
	n2 := *n
	n2.level++

	return &n2
}

func (n *Node) withRequired(b bool) *Node {
	n2 := *n
	n2.required = b

	return &n2
}

func (n *Node) withKey(k string) *Node {
	n2 := *n
	n2.key = k

	return &n2
}

func (n *Node) withPrefix(k string) *Node {
	n2 := *n
	n2.prefix = k

	return &n2
}

func (n *Node) inputLookup() *Node {
	n2 := *n
	n2.input = util.MapLookupPath(n.input, n.key)

	return &n2
}

func (n *Node) isRoot() bool {
	return n.key == "root"
}

func (n *Node) processObjectAnyOneOf(sch []*jsonschema.Schema, ret map[string]interface{}) error {
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

	vals, err := n.processObject(optsMap[opt], ret)
	if err != nil {
		return err
	}

	for k, v := range vals {
		ret[k] = v
	}

	return nil
}

func (n *Node) processObjectDependencies(sch *jsonschema.Schema, ret map[string]interface{}) error {
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

				vals, err := n.processObject(&vcopy, ret)
				if err != nil {
					return err
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

				val, err := n.withKey(propKey).withRequired(depReq[propKey]).process(prop.(*jsonschema.Schema))
				if err != nil {
					return err
				}

				ret[propKey] = val
			}
		}
	}

	return nil
}

func (n *Node) processObjectAdditionalProperties(sch *jsonschema.Schema, ret map[string]interface{}) error {
	props, ok := sch.AdditionalProperties.(*jsonschema.Schema)
	if !ok {
		return nil
	}

	props = schema(props)

	keyTitle := sch.Title
	if keyTitle == "" {
		keyTitle = n.key
	}

	keyTitle = fmt.Sprintf("%s%s", n.prefix, keyTitle)

	prefix := "Additional property"
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
			return err
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
			return err
		}

		if key == "" {
			return nil
		}

		val, err := n.withPrefix(fmt.Sprintf("%s value: ", prefix)).withKey(key).withRequired(false).process(props)
		if err != nil {
			if err == terminal.InterruptErr {
				return nil
			}

			return err
		}

		ret[key] = val
	}

	return nil
}

func (n *Node) processObject(sch *jsonschema.Schema, values map[string]interface{}) (map[string]interface{}, error) {
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

		v, err := n.withKey(k).withRequired(req[k]).process(val.(*jsonschema.Schema))
		if err != nil {
			return nil, err
		}

		if v != nil {
			ret[k] = v
		}
	}

	if len(sch.AnyOf) > 0 {
		err := n.processObjectAnyOneOf(sch.AnyOf, ret)
		if err != nil {
			return nil, err
		}
	}

	if len(sch.OneOf) > 0 {
		err := n.processObjectAnyOneOf(sch.OneOf, ret)
		if err != nil {
			return nil, err
		}
	}

	for _, of := range sch.AllOf {
		vals, err := n.processObject(of, ret)
		if err != nil {
			return nil, err
		}

		for k, v := range vals {
			ret[k] = v
		}
	}

	err := n.processObjectDependencies(sch, ret)
	if err != nil {
		return nil, err
	}

	// Parse additional properties if they exist.
	err = n.processObjectAdditionalProperties(sch, ret)

	return ret, err
}

func (n *Node) promptArrayStandard(keyTitle string, arraySch, itemSch *jsonschema.Schema) ([]interface{}, error) {
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
			Message: addLevelPrefix(n.level, keyTitle),
			Default: def,
			Help:    arraySch.Description,
			Options: selectOpts,
		}, &arrStr)
		if err != nil {
			return nil, err
		}

		switch typ {
		case JSONTypeInteger:
			ret = make([]interface{}, len(arrStr))
			for i, v := range arrStr {
				ret[i], _ = strconv.Atoi(v)
			}

			return ret, nil
		case JSONTypeNumber:
			ret = make([]interface{}, len(arrStr))
			for i, v := range arrStr {
				ret[i], _ = strconv.ParseFloat(v, 64)
			}

			return ret, nil
		case JSONTypeString:
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

		val, err := n.withPrefix(fmt.Sprintf("%s: ", n.prefix)).withRequired(false).process(itemSch)
		if err != nil {
			return nil, err
		}

		ret = append(ret, val)
	}

	return ret, nil
}

func (n *Node) promptArrayFixed(keyTitle string, arraySch *jsonschema.Schema, itemSch []*jsonschema.Schema) ([]interface{}, error) {
	var ret []interface{}

	for _, itm := range itemSch {
		itm = schema(itm)

		val, err := n.withPrefix(fmt.Sprintf("%s: ", n.prefix)).withRequired(false).process(itm)
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

			val, err := n.withRequired(false).process(itm)
			if err != nil {
				return nil, err
			}

			ret = append(ret, val)
		}
	}

	return ret, nil
}

func (n *Node) promptArray(sch *jsonschema.Schema) ([]interface{}, error) {
	sch = schema(sch)

	prefix := "Array value"
	if sch.Title != "" {
		prefix = sch.Title
	}

	keyTitle := sch.Title
	if keyTitle == "" {
		keyTitle = n.key
	}

	switch itemSch := sch.Items.(type) {
	case *jsonschema.Schema:
		// Standard array.
		return n.withPrefix(prefix).promptArrayStandard(keyTitle, sch, itemSch)

	case []*jsonschema.Schema:
		// Fixed items array.
		return n.withPrefix(prefix).promptArrayFixed(keyTitle, sch, itemSch)
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

func Prompt(level int, data []byte, input map[string]interface{}) (interface{}, error) {
	js := jsonschema.NewCompiler()
	js.Draft = jsonschema.Draft7
	js.ExtractAnnotations = true

	err := js.AddResource("schema.json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	sch, err := js.Compile("schema.json")
	if err != nil {
		return nil, err
	}

	typ := getSchemaType(schema(sch))
	if typ != JSONTypeObject {
		return nil, fmt.Errorf("invalid jsonschema prompt, expected an object got %s", typ)
	}

	n := &Node{
		level: level,
		key:   "root",
		input: input,
	}

	return n.process(sch)
}

func addLevelPrefix(level int, msg string) string {
	return fmt.Sprintf("%s %s", strings.Repeat("#", level+1), msg)
}

func (n *Node) process(sch *jsonschema.Schema) (interface{}, error) { // nolint:gocyclo
	sch = schema(sch)

	typ := getSchemaType(sch)
	if typ == "" && n.isRoot() {
		typ = JSONTypeObject
	}

	keyTitle := sch.Title
	if keyTitle == "" {
		keyTitle = n.key
	}

	keyTitle = fmt.Sprintf("%s%s", n.prefix, keyTitle)

	var opts []survey.AskOpt

	if n.required {
		opts = append(opts, survey.WithValidator(survey.Required))
	}

	inputVal, inputOk := n.input[n.key]

	switch typ {
	case JSONTypeObject:
		for {
			if !n.isRoot() {
				fmt.Println()

				n = n.inputLookup()
			}

			if sch.Title != "" {
				pterm.FgYellow.Println(addLevelPrefix(n.level, sch.Title))
			}

			if sch.Description != "" {
				pterm.FgGray.Println(addLevelPrefix(n.level, sch.Description))
			}

			val, err := n.increaseLevel().processObject(sch, nil)
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

	case JSONTypeArray:
		if val, ok := inputVal.([]interface{}); ok {
			err := validateValue(val, sch)
			if err != nil {
				return nil, err
			}

			return val, nil
		}

		if sch.Title != "" && !sch.UniqueItems {
			pterm.FgBlue.Println(addLevelPrefix(n.level, sch.Title))
		}

		if sch.Description != "" {
			pterm.FgGray.Println(addLevelPrefix(n.level, sch.Description))
		}

		for {
			val, err := n.increaseLevel().promptArray(sch)
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

	case JSONTypeString:
		if inputOk {
			if val, ok := inputVal.(string); ok {
				err := validateValue(val, sch)
				if err != nil {
					return nil, err
				}

				pterm.Printf("%s %s\n", pterm.Bold.Sprintf("%s:", keyTitle), val)

				return val, nil
			}
		}

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

		if o == "" {
			return nil, nil
		}

		return o, nil

	case JSONTypeNumber:
		if inputOk {
			if val, ok := inputVal.(float64); ok {
				err := validateValue(val, sch)
				if err != nil {
					return nil, err
				}

				pterm.Printf("%s %f\n", pterm.Bold.Sprintf("%s:", keyTitle), val)

				return val, nil
			}
		}

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

		if o == "" {
			return nil, nil
		}

		v, _ := strconv.ParseFloat(o, 64)

		return v, nil

	case JSONTypeInteger:
		if inputOk {
			if val, ok := inputVal.(int); ok {
				err := validateValue(val, sch)
				if err != nil {
					return nil, err
				}

				pterm.Printf("%s %d\n", pterm.Bold.Sprintf("%s:", keyTitle), val)

				return val, nil
			}
		}

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

		if o == "" {
			return nil, nil
		}

		v, _ := strconv.Atoi(o)

		return v, nil

	case JSONTypeBoolean:
		if inputOk {
			if val, ok := inputVal.(bool); ok {
				err := validateValue(val, sch)
				if err != nil {
					return nil, err
				}

				pterm.Printf("%s %t\n", pterm.Bold.Sprintf("%s:", keyTitle), val)

				return val, nil
			}
		}

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

	case JSONTypeNull:
		pterm.FgWhite.Println(keyTitle)

		if sch.Description != "" {
			pterm.FgGray.Println(sch.Description)
		}

		return nil, nil
	}

	panic("unknown type")
}

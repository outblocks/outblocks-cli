package lockfile

import (
	"fmt"
	"io/ioutil"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goccy/go-yaml"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"github.com/pterm/pterm"
)

type Lockfile struct {
	Plugins []*Plugin `json:"plugins,omitempty"`

	yamlPath string
	yamlData []byte
}

func (l *Lockfile) Validate() error {
	return validation.ValidateStruct(l)
}

func LoadLockfile(f string) (*Lockfile, error) {
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("cannot read lockfile: %w", err)
	}

	p, err := LoadLockfileData(f, data)
	if err != nil {
		return nil, err
	}

	return p, err
}

func LoadLockfileData(path string, data []byte) (*Lockfile, error) {
	out := &Lockfile{
		yamlPath: path,
		yamlData: data,
	}

	if err := yaml.UnmarshalWithOptions(data, out, yaml.Validator(validator.DefaultValidator()), yaml.UseJSONUnmarshaler(), yaml.Strict()); err != nil {
		return nil, fmt.Errorf("load lockfile %s error: \n%s", path, yaml.FormatError(err, pterm.PrintColor, true))
	}

	return out, nil
}

func (l *Lockfile) PluginByName(name string) *Plugin {
	if l == nil {
		return nil
	}

	for _, plug := range l.Plugins {
		if plug.Name == name {
			return plug
		}
	}

	return nil
}

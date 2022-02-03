package templating

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ansel1/merry/v2"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goccy/go-yaml"
	"github.com/goreleaser/fileglob"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/jsonschemaform"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/internal/validator"
	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/templates"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

const (
	TemplateYAMLName    = "template.outblocks"
	TemplateProjectName = "template.outblocks.project"
	TemplateValuesName  = "template.outblocks.values"
	TemplateValuesJSON  = "template.outblocks.values.json"
	TemplateProjectJSON = "template.outblocks.project.json"
)

var (
	TemplateTypes = []string{"project"}
)

var ErrMissingMetadata = errors.New("template metadata (template.outblocks.yaml) missing")

type Template struct {
	Type          string   `json:"type"`
	TemplateFiles []string `json:"template_files"`

	projectPrompt   []byte
	projectTemplate []byte
	valuesPrompt    []byte
	valuesTemplate  []byte

	Project *TemplateProject
	Values  *TemplateValues

	dir string
}

func (t *Template) Validate() error {
	return validation.ValidateStruct(t,
		validation.Field(&t.Type, validation.In(util.InterfaceSlice(TemplateTypes)...)),
	)
}

type TemplateProject struct {
	Plugins      []*config.Plugin              `json:"plugins"`
	Dependencies map[string]*config.Dependency `json:"dependencies"`
	DNS          []*config.DNS                 `json:"dns"`

	values map[string]interface{}
}

type TemplateValues struct {
	Val map[string]interface{}
}

func readAndRemoveFileIfExists(path string) ([]byte, error) {
	if !plugin_util.FileExists(path) {
		return nil, nil
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, merry.Errorf("cannot read file %s: %w", path, err)
	}

	_ = os.Remove(path)

	return contents, nil
}

func LoadTemplate(dir string) (*Template, error) {
	templateFile := fileutil.FindYAML(filepath.Join(dir, TemplateYAMLName))

	templateData, err := os.ReadFile(templateFile)
	if err != nil {
		return nil, merry.Errorf("cannot read template yaml: %w", err)
	}

	t := &Template{
		dir: dir,
	}

	if err := yaml.UnmarshalWithOptions(templateData, t, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, merry.Errorf("load template error: \n%s", yaml.FormatErrorDefault(err))
	}

	t.projectTemplate, err = readAndRemoveFileIfExists(fileutil.FindYAML(filepath.Join(t.dir, TemplateProjectName)))
	if err != nil {
		return nil, err
	}

	t.projectPrompt, err = readAndRemoveFileIfExists(filepath.Join(t.dir, TemplateProjectJSON))
	if err != nil {
		return nil, err
	}

	t.valuesTemplate, err = readAndRemoveFileIfExists(fileutil.FindYAML(filepath.Join(t.dir, TemplateValuesName)))
	if err != nil {
		return nil, err
	}

	t.valuesPrompt, err = readAndRemoveFileIfExists(filepath.Join(t.dir, TemplateValuesJSON))
	if err != nil {
		return nil, err
	}

	_ = os.Remove(templateFile)

	return t, nil
}

func promptData(path string, d []byte) (map[string]interface{}, error) {
	data, err := jsonschemaform.Prompt(1, d)
	if err != nil {
		return nil, merry.Errorf("error processing jsonschema file %s: %w", path, err)
	}

	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, merry.Errorf("jsonschema file %s does not return object", path)
	}

	return m, nil
}

func (t *Template) HasProjectPrompt() bool {
	return len(t.projectPrompt) != 0
}

func (t *Template) ParseProjectTemplate(projectName string) error {
	t.Project = &TemplateProject{
		values: map[string]interface{}{
			"project": map[string]interface{}{
				"name": projectName,
			},
		},
	}

	// Prompt data.
	var (
		data map[string]interface{}
		err  error
	)

	if len(t.projectPrompt) != 0 {
		data, err = promptData(filepath.Join(t.dir, TemplateProjectJSON), t.projectPrompt)
		if err != nil {
			return err
		}
	}

	// Load template.
	if len(t.projectTemplate) == 0 {
		return nil
	}

	tmpl, err := templates.LoadTemplate(TemplateProjectName).Parse(string(t.projectTemplate))
	if err != nil {
		return merry.Errorf("error parsing project template: %w", err)
	}

	// Process template.
	var buf bytes.Buffer

	t.Project.values = plugin_util.MergeMaps(data, t.Project.values)

	err = tmpl.Execute(&buf, t.Project.values)
	if err != nil {
		return merry.Errorf("error processing project template: %w", err)
	}

	if err := yaml.UnmarshalWithOptions(buf.Bytes(), t.Project, yaml.Validator(validator.DefaultValidator())); err != nil {
		return merry.Errorf("load project template error: \n%s", yaml.FormatErrorDefault(err))
	}

	curDir, err := os.Getwd()
	if err != nil {
		return merry.Errorf("can't get current working dir: %w", err)
	}

	path, err := filepath.Rel(curDir, t.dir)
	if err != nil {
		return merry.Errorf("can't create relative path: %w", err)
	}

	files, err := fileglob.Glob(filepath.Join(path, fmt.Sprintf("{%s}", strings.Join(t.TemplateFiles, ","))))
	if err != nil {
		return merry.Errorf("error matching template files: %w", err)
	}

	for _, file := range files {
		var buf bytes.Buffer

		tmpl, err := templates.LoadTemplate(filepath.Base(file)).ParseFiles(file)
		if err != nil {
			return merry.Errorf("error processing template file %s: %w", file, err)
		}

		err = tmpl.Execute(&buf, t.Project.values)
		if err != nil {
			return merry.Errorf("error matching template file %s: %w", file, err)
		}

		err = os.WriteFile(file, buf.Bytes(), 0)
		if err != nil {
			return merry.Errorf("error saving template file %s: %w", file, err)
		}
	}

	return nil
}

func (t *Template) HasValuesPrompt() bool {
	return len(t.valuesPrompt) != 0
}

func (t *Template) ParseValuesTemplate() ([]byte, error) {
	t.Values = &TemplateValues{}

	// Prompt data.
	var (
		data map[string]interface{}
		err  error
	)

	if len(t.valuesPrompt) != 0 {
		data, err = promptData(filepath.Join(t.dir, TemplateValuesJSON), t.valuesPrompt)
		if err != nil {
			return nil, err
		}
	}

	// Load template.
	if len(t.valuesTemplate) == 0 {
		return nil, nil
	}

	tmpl, err := templates.LoadTemplate(TemplateValuesName).Parse(string(t.valuesTemplate))
	if err != nil {
		return nil, merry.Errorf("error parsing values template: %w", err)
	}

	// Process template.
	var buf bytes.Buffer

	data = plugin_util.MergeMaps(data, t.Project.values)

	err = tmpl.Execute(&buf, data)
	if err != nil {
		return nil, merry.Errorf("error processing values template: %w", err)
	}

	if err := yaml.UnmarshalWithOptions(buf.Bytes(), &t.Values.Val, yaml.Validator(validator.DefaultValidator())); err != nil {
		return nil, merry.Errorf("load values template error: \n%s", yaml.FormatErrorDefault(err))
	}

	return buf.Bytes(), nil
}

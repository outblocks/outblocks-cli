package templates

import (
	_ "embed"
	"strings"
	"text/template"

	"github.com/23doors/go-yaml"
	"github.com/Masterminds/sprig"
)

var (
	//go:embed project.yaml.tpl
	ProjectYAML string

	//go:embed values.yaml.tpl
	ValuesYAML string

	//go:embed app-static.yaml.tpl
	StaticAppYAML string
	//go:embed app-service.yaml.tpl
	ServiceAppYAML string
	//go:embed app-function.yaml.tpl
	FunctionAppYAML string
)

func funcMap() template.FuncMap {
	return template.FuncMap{
		"toYaml": toYaml,
	}
}

func toYaml(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		return ""
	}

	return strings.TrimRight(string(data), " \n")
}

func LoadTemplate(name string) *template.Template {
	return template.New(name).Funcs(sprig.TxtFuncMap()).Funcs(funcMap()).Option("missingkey=zero")
}

func lazyInit(name, tmpl string) func() *template.Template {
	var templ *template.Template

	return func() *template.Template {
		if templ == nil {
			var err error

			templ, err = LoadTemplate(name).Parse(tmpl)
			if err != nil {
				panic(err)
			}
		}

		return templ
	}
}

var (
	ProjectYAMLTemplate     = lazyInit("project.yaml", ProjectYAML)
	ValuesYAMLTemplate      = lazyInit("values.yaml", ValuesYAML)
	StaticAppYAMLTemplate   = lazyInit("static_app.yaml", StaticAppYAML)
	ServiceAppYAMLTemplate  = lazyInit("service_app.yaml", ServiceAppYAML)
	FunctionAppYAMLTemplate = lazyInit("function_app.yaml", FunctionAppYAML)
)

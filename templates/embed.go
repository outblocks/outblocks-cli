package templates

import (
	_ "embed"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/goccy/go-yaml"
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

	return string(data)
}

func lazyInit(name, tmpl string) func() *template.Template {
	var templ *template.Template

	return func() *template.Template {
		if templ == nil {
			templ = template.Must(template.New(name).Funcs(sprig.TxtFuncMap()).Funcs(funcMap()).Parse(tmpl))
		}

		return templ
	}
}

var (
	ProjectYAMLTemplate    = lazyInit("project.yaml", ProjectYAML)
	ValuesYAMLTemplate     = lazyInit("values.yaml", ValuesYAML)
	StaticAppYAMLTemplate  = lazyInit("static_app.yaml", StaticAppYAML)
	ServiceAppYAMLTemplate = lazyInit("service_app.yaml", ServiceAppYAML)
)

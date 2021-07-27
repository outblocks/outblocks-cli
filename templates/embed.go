package templates

import _ "embed"

var (
	//go:embed project.yaml.tpl
	ProjectYAML string

	//go:embed values.yaml.tpl
	ValuesYAML string

	//go:embed app-static.yaml.tpl
	StaticAppYAML string
)

package templates

import _ "embed"

var (
	//go:embed project.yaml.go.tpl
	ProjectYAML string

	//go:embed values.yaml.go.tpl
	ValuesYAML string
)

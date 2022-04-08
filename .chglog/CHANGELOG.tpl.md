# {{ .Info.Title }}
{{- range .Versions }}

## {{ if .Tag.Previous }}[{{ .Tag.Name }}]({{ $.Info.RepositoryURL }}/compare/{{ .Tag.Previous.Name }}...{{ .Tag.Name }}){{ else }}{{ .Tag.Name }}{{ end }}

> {{ ternary now .Tag.Date (eq (.Tag.Date | datetime "2006") "0001") | datetime "2006-01-02" }}

{{- range .CommitGroups }}

### {{ .Title }}
{{- range .Commits }}

[`{{ .Hash.Short }}`]({{ $.Info.RepositoryURL }}/commit/{{ .Hash.Long }}) {{ if .Scope }}**{{ .Scope }}:** {{ end }}{{ .Subject }}
{{- end -}}
{{- end -}}
{{- end }}

{{- if .DNSDomain }}
# Base DNS domain of environment.
base_url: {{.DNSDomain}}

{{ end -}}
{{- if .TemplateValues -}}
# Template values.
{{ .TemplateValues | toString }}

{{- end -}}
{{- if .PluginValues }}
# Plugin specific values
{{- end }}
{{ .PluginValues | toYaml }}

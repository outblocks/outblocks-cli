# Static app config.

# You can use ${var.*} expansion to source it from values.yaml per environment,
# e.g. url: ${var.base_url}/app1/

# Name of the app.
name: {{.App.Name}}
{{- if .Type }}

# App type.
type: {{.Type}}
{{- end }}

# URL of the app.
url: {{.URL}}

# Build defines where static files are stored and optionally which command should be used to generate them.
build:
{{- if .App.Build.Command }}
  # Command to be run to generate output files.
  command: {{.App.Build.Command}}
{{ end }}
  # Directory where generated files will end up.
  dir: {{.App.Build.Dir}}

# Routing to be used:
#   'react' for react browser routing.
#   'disabled' for no additional routing.
routing: {{.App.Routing}}

# Static app config.

# You can use ${var.*} expansion to source it from values.yaml per environment,
# e.g. url: ${var.base_url}/app1/

# Name of the app.
name: {{.App.Name}}
{{- if .Type }}

# Type of the app.
type: {{.Type}}
{{- end }}

# URL of the app.
url: {{.URL}}
# Path redirect rewrites URL to specified path. URL path from 'url' field will be stripped and replaced with value below.
# '/' should be fine for most apps.
pathRedirect: /

# Build defines where static files are stored and optionally which command should be used to generate them.
build:
{{- if .App.Build.Command }}
  # Command to be run to generate output files.
  command: {{.App.Build.Command}}
{{ end }}
  # Directory where generated files will end up.
  dir: {{.App.Build.Dir}}

# Run defines where how development is handled of application during `ok run`.
run:
{{- if .App.RunInfo.Command }}
  # Command to be run to for dev mode.
  command: {{.App.RunInfo.Command}}
{{ end }}
  # Additional environment variables to pass.
  # env:
  #   BROWSER: none  # disable opening browser for react app

  # Port override, by default just assigns next port starting from listen-port.
  # port: 8123

# Routing to be used:
#   'react' for react browser routing.
#   'disabled' for no additional routing.
routing: {{.App.Routing}}

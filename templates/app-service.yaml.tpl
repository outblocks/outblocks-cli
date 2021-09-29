# Service app config.

# You can use ${var.*} expansion to source it from values.yaml per environment,
# e.g. url: ${var.base_url}/app1/

# Name of the app.
name: {{.App.Name}}

# Working directory of the app where all commands will be run. All other dirs will be relative to this one.
dir: {{.App.Dir}}

# Type of the app.
type: {{.App.Type}}

# URL of the app.
url: {{.App.AppURL}}
# Path redirect rewrites URL to specified path. URL path from 'url' field will be stripped and replaced with value below.
# '/' should be fine for most apps.
pathRedirect: /

# Build defines how docker image will be built for this application.
build:
  # Dockerfile name to be used.
  dockerfile: {{.App.Build.Dockerfile}}
  # Directory which is used for dockerfile context.
  context: {{.App.Build.DockerContext}}

# Deploy defines where how deployment is handled of application during `ok deploy`.
deploy:
  plugin: {{.App.DeployInfo.Plugin}}

# Run defines where how development is handled of application during `ok run`.
run:
  plugin: {{.App.RunInfo.Plugin}}
{{- if .App.RunInfo.Command }}
  # Command to be run to for dev mode.
  command: {{.App.RunInfo.Command}}
{{ end }}
  # Additional environment variables to pass.
  # env:
  #   BROWSER: none  # disable opening browser for react app

  # Port override, by default just assigns next port starting from listen-port.
  # port: 8123

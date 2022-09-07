# Function app config.

# You can use ${var.*} expansion to source it from values.yaml per environment,
# e.g. url: ${var.base_url}/app1/

# Name of the app.
name: {{.App.Name}}
{{- if .App.Runtime }}

# The runtime in which the function is going to run, refer to cloud provider docs for possible options.
runtime: {{.App.Runtime}}
{{ end }}
# Name of the function that will be executed when the function is triggered.
entrypoint: {{.App.Entrypoint}}

# Working directory of the app where all commands will be run. All other dirs will be relative to this one.
dir: {{.App.Dir}}

# Type of the app.
type: {{.App.Type}}

# URL of the app.
url: {{.App.AppURL}}
# Path redirect rewrites URL to specified path.
# URL path from 'url' field will be stripped and replaced with value below.
# '/' should be fine for most apps.
path_redirect: /
# If app is not meant to be accessible without auth, mark it as private.
# private: true

# Deploy defines where how deployment is handled of application during `ok deploy`.
deploy:
  plugin: {{.App.DeployInfo.Plugin}}

# Run defines where how development is handled of application during `ok run`.
run:
  plugin: {{.App.RunInfo.Plugin}}
{{- if .App.RunInfo.Command }}
  # Command to be run to for dev mode.
  command: {{.App.RunInfo.Command | toJson }}
{{ end }}
  # Additional environment variables to pass.
  # env:
  #   BROWSER: none  # disable opening browser for react app

  # Port override, by default just assigns next port starting from listen-port.
  # port: 8123

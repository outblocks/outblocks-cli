# Project config.

# You can use ${var.*} expansion to source it from values.yaml per environment,
# e.g. domain: ${var.base_url}

# Name of the project.
name: {{.Name}}

# State - where project state will be stored.
state:
  type: {{.State.Type}}

# Main base domain for apps - loaded from values.yaml for easy override per environment.
dns:
  - domain: ${var.base_url}

# Plugins that will be used for running, deployment etc.
plugins:
{{ .Plugins | toYaml | indent 2 -}}

package plugins

import validation "github.com/go-ozzo/ozzo-validation/v4"

type PluginType struct {
	Type  string       `json:"type"`
	Match *PluginMatch `json:"match"`
}

func (p *PluginType) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.Type, validation.Required),
	)
}

type PluginMatch struct {
	Deploy string                 `json:"deploy"`
	Other  map[string]interface{} `yaml:"-,remain"`
}

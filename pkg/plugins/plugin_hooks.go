package plugins

import validation "github.com/go-ozzo/ozzo-validation/v4"

type PluginHooks struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func (p *PluginHooks) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.Type, validation.Required),
		validation.Field(&p.Name, validation.Required),
	)
}

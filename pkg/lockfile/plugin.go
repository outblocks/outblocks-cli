package lockfile

import (
	"github.com/Masterminds/semver"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type Plugin struct {
	Name    string          `json:"name" valid:"required"`
	Version *semver.Version `json:"version" valid:"required"`
	Source  string          `json:"source" valid:"required"`
}

func (p *Plugin) Matches(name string, ver *semver.Version, source string) bool {
	return p.Name == name && p.Version.Equal(ver) && p.Source == source
}

func (p *Plugin) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.Name, validation.Required),
		validation.Field(&p.Version, validation.Required),
		validation.Field(&p.Source, validation.Required),
	)
}

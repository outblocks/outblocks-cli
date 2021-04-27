package lockfile

import (
	"github.com/blang/semver/v4"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/outblocks/outblocks-cli/internal/version"
)

type Plugin struct {
	Name    string                 `json:"name" valid:"required"`
	Version *version.SemverVersion `json:"version" valid:"required"`
	Source  string                 `json:"source" valid:"required"`
}

func (p *Plugin) Matches(name string, ver *semver.Version, source string) bool {
	return p.Name == name && p.Version.Version.EQ(*ver) && p.Source == source
}

func (p *Plugin) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.Name, validation.Required),
		validation.Field(&p.Version, validation.Required),
		validation.Field(&p.Source, validation.Required),
	)
}

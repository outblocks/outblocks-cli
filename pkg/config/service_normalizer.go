package config

import (
	"fmt"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type ServiceConfigNormalizer struct {
	*ServiceConfig
	plugins []*plugins.Plugin
}

func NewServiceConfigNormalizer(cfg *ServiceConfig, p []*plugins.Plugin) *ServiceConfigNormalizer {
	return &ServiceConfigNormalizer{ServiceConfig: cfg, plugins: p}
}

// Initial first pass validation.
func (f *ServiceConfigNormalizer) Normalize() error {
	err := func() error {
		return nil
	}()

	if err != nil {
		return fmt.Errorf("service config validation failed.\nfile: %s\n%s", f.Path, err)
	}

	return nil
}

func (f *ServiceConfigNormalizer) yamlError(path, s string) error {
	return fileutil.YAMLError(path, s, f.data)
}

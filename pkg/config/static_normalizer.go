package config

import (
	"fmt"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type StaticConfigNormalizer struct {
	*StaticConfig
	plugins []*plugins.Plugin
}

func NewStaticConfigNormalizer(cfg *StaticConfig, p []*plugins.Plugin) *StaticConfigNormalizer {
	return &StaticConfigNormalizer{StaticConfig: cfg, plugins: p}
}

// Initial first pass validation.
func (f *StaticConfigNormalizer) Normalize() error {
	err := func() error {
		return nil
	}()

	if err != nil {
		return fmt.Errorf("static config validation failed.\nfile: %s\n%s", f.Path, err)
	}

	return nil
}

func (f *StaticConfigNormalizer) yamlError(path, s string) error {
	return fileutil.YAMLError(path, s, f.data)
}

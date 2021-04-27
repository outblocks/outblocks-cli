package config

import (
	"fmt"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
)

type FunctionConfigNormalizer struct {
	*FunctionConfig
	plugins []*plugins.Plugin
}

func NewFunctionConfigNormalizer(cfg *FunctionConfig, p []*plugins.Plugin) *FunctionConfigNormalizer {
	return &FunctionConfigNormalizer{FunctionConfig: cfg, plugins: p}
}

// Initial first pass validation.
func (f *FunctionConfigNormalizer) Normalize() error {
	err := func() error {
		return nil
	}()

	if err != nil {
		return fmt.Errorf("function config validation failed.\nfile: %s\n%s", f.Path, err)
	}

	return nil
}

func (f *FunctionConfigNormalizer) yamlError(path, s string) error {
	return fileutil.YAMLError(path, s, f.data)
}

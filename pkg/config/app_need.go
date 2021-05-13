package config

import (
	"fmt"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-plugin-go/types"
)

type AppNeed struct {
	Other map[string]interface{} `yaml:"-,remain"`

	dep *Dependency
}

func (n *AppNeed) Normalize(name string, cfg *ProjectConfig, data []byte) error {
	dep := cfg.FindDependency(name)
	if dep == nil {
		return fileutil.YAMLError(fmt.Sprintf("$.needs.%s", name), "object not found in project dependencies", data)
	}

	n.dep = dep

	return nil
}

func (n *AppNeed) PluginType() *types.AppNeed {
	return &types.AppNeed{
		Properties: n.Other,
	}
}

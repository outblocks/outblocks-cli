package config

import (
	"fmt"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

type AppNeed struct {
	Other map[string]interface{} `yaml:"-,remain"`

	dep *Dependency
}

func (n *AppNeed) Normalize(name string, cfg *Project, data []byte) error {
	dep := cfg.DependencyByName(name)
	if dep == nil {
		return fileutil.YAMLError(fmt.Sprintf("$.needs.%s", name), "object not found in project dependencies", data)
	}

	n.dep = dep

	return nil
}

func (n *AppNeed) Dependency() *Dependency {
	return n.dep
}

func (n *AppNeed) Proto() *apiv1.AppNeed {
	return &apiv1.AppNeed{
		Dependency: n.dep.Name,
		Properties: plugin_util.MustNewStruct(n.Other),
	}
}

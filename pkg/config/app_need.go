package config

import (
	"fmt"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
)

type Need struct {
	Other map[string]interface{} `yaml:"-,remain"`

	dep *ProjectDependency
}

func (n *Need) Normalize(name string, cfg *ProjectConfig, data []byte) error {
	dep := cfg.FindDependency(name)
	if dep == nil {
		return fileutil.YAMLError(fmt.Sprintf("$.needs.%s", name), "object not found in project dependencies", data)
	}

	n.dep = dep

	return nil
}

package plugins

import (
	"fmt"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
)

type PluginNormalizer struct {
	*Plugin
}

var (
	pluginTypes = map[string]Action{
		"deploy": ActionDeploy,
		"run":    ActionRun,
	}
)

func NewPluginNormalizer(p *Plugin) *PluginNormalizer {
	return &PluginNormalizer{p}
}

func (p *PluginNormalizer) Normalize() error {
	err := func() error {
		p.actions = make([]Action, len(p.Actions))

		for i, typ := range p.Actions {
			t, ok := pluginTypes[strings.ToLower(typ)]
			if !ok {
				return p.yamlError(fmt.Sprintf("$.actions[%d]", i), "Plugin.Action is invalid")
			}

			p.actions[i] = t
		}

		return nil
	}()

	if err != nil {
		return fmt.Errorf("plugin config validation failed.\nfile: %s\n%s", p.yamlPath, err)
	}

	return nil
}

func (p *PluginNormalizer) yamlError(path, s string) error {
	return fileutil.YAMLError(path, s, p.data)
}

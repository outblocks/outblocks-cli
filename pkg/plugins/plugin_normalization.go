package plugins

import (
	"fmt"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
)

var (
	pluginTypes = map[string]Action{
		"dns":    ActionDNS,
		"deploy": ActionDeploy,
		"run":    ActionRun,
	}
)

func (p *Plugin) Normalize() error {
	err := func() error {
		p.actions = make([]Action, len(p.Actions))

		for i, typ := range p.Actions {
			t, ok := pluginTypes[strings.ToLower(typ)]
			if !ok {
				return p.yamlError(fmt.Sprintf("$.actions[%d]", i), "plugin.action is invalid")
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

func (p *Plugin) yamlError(path, msg string) error {
	return fileutil.YAMLError(path, msg, p.yamlData)
}

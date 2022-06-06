package plugins

import (
	"fmt"
	"strings"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
)

var (
	pluginTypes = map[string]Action{
		"dns":         ActionDNS,
		"deploy":      ActionDeploy,
		"run":         ActionRun,
		"lock":        ActionLock,
		"state":       ActionState,
		"deploy_hook": ActionDeployHook,
		"secrets":     ActionSecrets,
	}
	pluginTypesIgnored = map[string]struct{}{
		"locking": {},
	}
)

func (p *Plugin) Normalize() error {
	err := func() error {
		p.actions = make([]Action, len(p.Actions))

		for i, typ := range p.Actions {
			t, ok := pluginTypes[strings.ToLower(typ)]
			if _, ignored := pluginTypesIgnored[typ]; !ok && !ignored {
				return p.yamlError(fmt.Sprintf("$.actions[%d]", i), "plugin.action is invalid")
			}

			p.actions[i] = t
		}

		return nil
	}()

	if err != nil {
		return merry.Errorf("plugin config validation failed.\nfile: %s\n%s", p.yamlPath, err)
	}

	return nil
}

func (p *Plugin) yamlError(path, msg string) error {
	return fileutil.YAMLError(path, msg, p.yamlData)
}

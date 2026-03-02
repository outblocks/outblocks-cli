package cmd

import (
	"testing"

	"github.com/outblocks/outblocks-cli/pkg/config"
)

func TestAddPluginsCommandsSkipsUnloadedPlugins(t *testing.T) {
	e := NewExecutor()
	e.cfg = &config.Project{
		Plugins: []*config.Plugin{{}},
	}

	if err := e.addPluginsCommands(); err != nil {
		t.Fatalf("addPluginsCommands returned error: %v", err)
	}
}

package actions

import (
	"context"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins"
	"github.com/outblocks/outblocks-cli/pkg/plugins/client"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
)

type Command struct {
	log  logger.Logger
	cfg  *config.Project
	opts *CommandOptions
}

type CommandOptions struct {
	Name       string
	Plugin     *plugins.Plugin
	InputTypes []plugins.CommandInputType
	Args       *apiv1.CommandArgs
}

func NewCommand(log logger.Logger, cfg *config.Project, opts *CommandOptions) *Command {
	return &Command{
		log:  log,
		cfg:  cfg,
		opts: opts,
	}
}

func (c *Command) Run(ctx context.Context) error {
	yamlContext := &client.YAMLContext{
		Prefix: "$.state",
		Data:   c.cfg.YAMLData(),
	}

	// Get state.
	state, _, err := getState(ctx, c.cfg.State, false, 0, true, yamlContext)
	if err != nil {
		return err
	}

	req := &apiv1.CommandRequest{
		Command: c.opts.Name,
		Args:    c.opts.Args,
	}

	for _, t := range c.opts.InputTypes {
		switch t {
		case plugins.CommandInputTypeAppStates:
			req.AppStates = state.Apps
		case plugins.CommandInputTypeDependencyStates:
			req.DependencyStates = state.Dependencies
		case plugins.CommandInputTypePluginState:
			req.PluginState = state.Plugins[c.opts.Plugin.Name].Proto()
		}
	}

	return c.opts.Plugin.Client().Command(ctx, req)
}

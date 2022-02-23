package actions

import (
	"context"
	"fmt"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins/client"
)

type Status struct {
	log logger.Logger
	cfg *config.Project
}

func NewStatus(log logger.Logger, cfg *config.Project) *Status {
	return &Status{
		log: log,
		cfg: cfg,
	}
}

func (d *Status) Run(ctx context.Context) error {
	yamlContext := &client.YAMLContext{
		Prefix: "$.state",
		Data:   d.cfg.YAMLData(),
	}

	// Get state.
	state, _, err := getState(ctx, d.cfg.State, false, 0, true, yamlContext)
	if err != nil {
		return err
	}

	// get state
	// plan with all from state
	fmt.Println(state)

	return nil
}

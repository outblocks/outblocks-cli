package actions

import (
	"context"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
)

type ForceUnlock struct {
	log logger.Logger
}

func NewForceUnlock(log logger.Logger) *ForceUnlock {
	return &ForceUnlock{
		log: log,
	}
}

func (f *ForceUnlock) Run(ctx context.Context, cfg *config.Project, lockinfo string) error {
	state := cfg.State

	return state.Plugin().Client().ForceUnlock(ctx, state.Type, state.Env, state.Other, lockinfo)
}

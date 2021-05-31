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

func (f *ForceUnlock) Run(ctx context.Context, cfg *config.Project, lockID string) error {
	return releaseLock(ctx, cfg, lockID)
}

func releaseLock(ctx context.Context, cfg *config.Project, lockID string) error {
	state := cfg.State

	if state.IsLocal() {
		return nil
	}

	return state.Plugin().Client().ReleaseLock(ctx, state.Type, state.Env, state.Other, lockID)
}

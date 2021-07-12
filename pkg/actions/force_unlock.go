package actions

import (
	"context"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins/client"
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
	return releaseLock(cfg, lockID)
}

func releaseLock(cfg *config.Project, lockID string) error {
	if lockID == "" {
		return nil
	}

	state := cfg.State

	if state.IsLocal() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultTimeout)
	defer cancel()

	return state.Plugin().Client().ReleaseLock(ctx, state.Type, state.Env, state.Other, lockID)
}

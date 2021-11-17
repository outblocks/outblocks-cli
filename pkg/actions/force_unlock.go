package actions

import (
	"context"

	"github.com/outblocks/outblocks-cli/pkg/config"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	"github.com/outblocks/outblocks-cli/pkg/plugins/client"
)

type ForceUnlock struct {
	log logger.Logger
	cfg *config.Project
}

func NewForceUnlock(log logger.Logger, cfg *config.Project) *ForceUnlock {
	return &ForceUnlock{
		cfg: cfg,
		log: log,
	}
}

func (f *ForceUnlock) Run(ctx context.Context, lockinfo string) error {
	return releaseStateLock(f.cfg, lockinfo)
}

func releaseStateLock(cfg *config.Project, lockinfo string) error {
	if lockinfo == "" {
		return nil
	}

	state := cfg.State

	if state.IsLocal() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultTimeout)
	defer cancel()

	return state.Plugin().Client().ReleaseStateLock(ctx, state.Type, state.Other, lockinfo)
}

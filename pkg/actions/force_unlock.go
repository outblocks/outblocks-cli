package actions

import (
	"context"
	"strings"

	"github.com/ansel1/merry/v2"
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
	locks := strings.Split(lockinfo, ",")

	if len(locks) == 0 {
		return nil
	}

	if len(locks) == 1 && !strings.Contains(locks[0], "=") {
		return releaseStateLock(f.cfg, lockinfo)
	}

	lockMap := make(map[string]string)

	for _, l := range locks {
		locksplit := strings.SplitN(l, "=", 2)
		if len(locksplit) != 2 {
			return merry.New("invalid lock format")
		}

		lockMap[locksplit[0]] = locksplit[1]
	}

	return releaseLocks(f.cfg, lockMap)
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

func releaseLocks(cfg *config.Project, locks map[string]string) error {
	if len(locks) == 0 {
		return nil
	}

	state := cfg.State

	if state.IsLocal() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultTimeout)
	defer cancel()

	return state.Plugin().Client().ReleaseLocks(ctx, state.Other, locks)
}

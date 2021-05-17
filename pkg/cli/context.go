package cli

import (
	"context"

	"github.com/outblocks/outblocks-cli/pkg/logger"
)

type Context struct {
	Ctx context.Context
	Log *logger.Log
}

func (c *Context) Debug() bool {
	return c.Log.Level() == logger.LogLevelDebug
}

func (c *Context) WithContext(ctx context.Context) *Context {
	c2 := *c
	c2.Ctx = ctx

	return &c2
}

func (c *Context) SetContext(ctx context.Context) {
	c.Ctx = ctx
}

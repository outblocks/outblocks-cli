package client

import (
	"errors"
	"fmt"
)

func IsPluginError(err error) bool {
	var e *PluginError

	return errors.As(err, &e)
}

type PluginError struct {
	c       *Client
	msg     string
	wrapped error
}

func NewPluginError(c *Client, msg string, wrapped error) *PluginError {
	return &PluginError{
		c:       c,
		msg:     msg,
		wrapped: wrapped,
	}
}

func (e *PluginError) Unwrap() error {
	return e.wrapped
}

func (e *PluginError) Error() string {
	if e.wrapped != nil {
		return fmt.Sprintf("plugin '%s' %s: %s", e.c.name, e.msg, e.wrapped)
	}

	return fmt.Sprintf("plugin '%s' %s", e.c.name, e.msg)
}

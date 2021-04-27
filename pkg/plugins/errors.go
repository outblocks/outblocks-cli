package plugins

import "errors"

var (
	ErrPluginNotFound               = errors.New("plugin not found")
	ErrPluginNoMatchingVersionFound = errors.New("no matching version for plugin not found")
)

package config

import "errors"

var (
	ErrProjectConfigNotFound = errors.New("cannot find project config file, did you forget to initialize? run:\nok init")
)

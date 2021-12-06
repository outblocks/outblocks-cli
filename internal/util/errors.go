package util

import (
	"errors"

	"google.golang.org/grpc/status"
)

func StatusFromError(err error) (s *status.Status, ok bool) {
	for {
		s, ok = status.FromError(err)
		if ok {
			return s, ok
		}

		err = errors.Unwrap(err)
		if err == nil {
			return nil, false
		}
	}
}

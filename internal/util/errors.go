package util

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func StatusFromError(err error) (s *status.Status, ok bool) {
	for {
		s, ok = statusFromError(err)
		if ok {
			return s, ok
		}

		err = errors.Unwrap(err)
		if err == nil {
			return nil, false
		}
	}
}

func statusFromError(err error) (s *status.Status, ok bool) {
	if err == nil {
		return nil, true
	}

	type grpcstatus interface{ GRPCStatus() *status.Status }

	if gs, ok := err.(grpcstatus); ok {
		grpcStatus := gs.GRPCStatus()
		if grpcStatus == nil {
			// Error has status nil, which maps to codes.OK. There
			// is no sensible behavior for this, so we turn it into
			// an error with codes.Unknown and discard the existing
			// status.
			return status.New(codes.Unknown, err.Error()), false
		}

		return grpcStatus, true
	}

	var gs grpcstatus

	if errors.As(err, &gs) {
		grpcStatus := gs.GRPCStatus()
		if grpcStatus == nil {
			// Error wraps an error that has status nil, which maps
			// to codes.OK.  There is no sensible behavior for this,
			// so we turn it into an error with codes.Unknown and
			// discard the existing status.
			return status.New(codes.Unknown, err.Error()), false
		}

		p := grpcStatus.Proto()
		if p.Message == "" {
			p.Message = err.Error()
		}

		return status.FromProto(p), true
	}

	return status.New(codes.Unknown, err.Error()), false
}

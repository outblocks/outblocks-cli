package client

import (
	"regexp"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	comm "github.com/outblocks/outblocks-plugin-go"
)

func ValidateHandshake(h *comm.Handshake) error {
	return validation.ValidateStruct(h,
		validation.Field(&h.Protocol, validation.Required, validation.Match(regexp.MustCompile(`^(v1)$`))),
		validation.Field(&h.Addr, validation.Required),
	)
}

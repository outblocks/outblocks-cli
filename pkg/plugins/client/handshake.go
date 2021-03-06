package client

import (
	"regexp"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

func ValidateHandshake(h *plugin_go.Handshake) error {
	return validation.ValidateStruct(h,
		validation.Field(&h.Protocol, validation.Required, validation.Match(regexp.MustCompile(`^(v1)$`))),
		validation.Field(&h.Addr, validation.Required),
	)
}

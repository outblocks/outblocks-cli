package client

import (
	"strings"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

type YAMLContext struct {
	Prefix string
	Data   []byte
}

func (c *YAMLContext) Error(r *plugin_go.ValidationErrorResponse) error {
	return fileutil.YAMLError(strings.Join([]string{c.Prefix, r.Path}, "."), r.Error, c.Data)
}

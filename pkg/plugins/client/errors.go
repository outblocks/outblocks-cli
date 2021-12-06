package client

import (
	"errors"
	"fmt"
	"strings"

	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"google.golang.org/grpc/codes"
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

func (c *Client) newPluginError(msg string, wrapped error) *PluginError {
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

func (c *Client) mapErrorWithContext(msg string, err error, yamlContext *YAMLContext) error {
	if err == nil {
		return nil
	}

	st, ok := util.StatusFromError(err)
	if !ok {
		return c.newPluginError(msg, err)
	}

	for _, det := range st.Details() {
		switch r := det.(type) {
		case *apiv1.LockError:
			return fmt.Errorf("lock already acquired by %s at %s, to force unlock run:\nok force-unlock %s=%s",
				r.Owner, r.CreatedAt.AsTime(), r.LockName, r.LockInfo)
		case *apiv1.ValidationError:
			return fileutil.YAMLError(strings.Join([]string{yamlContext.Prefix, r.Path}, "."), r.Message, yamlContext.Data)
		}
	}

	if st.Code() == codes.Unknown {
		err = errors.New(st.Message())
	}

	return c.newPluginError(msg, err)
}

func (c *Client) mapError(msg string, err error) error {
	return c.mapErrorWithContext(msg, err, &c.yamlContext)
}

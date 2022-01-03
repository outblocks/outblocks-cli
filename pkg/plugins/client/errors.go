package client

import (
	"errors"
	"fmt"
	"io"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"google.golang.org/grpc/codes"
)

type PluginError struct {
	c       *Client
	msg     string
	wrapped error
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

func (c *Client) newPluginError(msg string, wrapped error) *PluginError {
	return &PluginError{
		c:       c,
		msg:     msg,
		wrapped: wrapped,
	}
}

func (c *Client) mapErrorWithContext(msg string, err error, yamlContext *YAMLContext) error {
	if err == nil || errors.Is(err, io.EOF) {
		return nil
	}

	st, ok := util.StatusFromError(err)
	if !ok {
		return c.newPluginError(msg, err)
	}

	for _, det := range st.Details() {
		switch r := det.(type) {
		case *apiv1.LockError:
			return merry.Errorf("lock already acquired by %s at %s, to force unlock run:\nok force-unlock --env %s %s=%s",
				c.env, r.Owner, r.CreatedAt.AsTime(), r.LockName, r.LockInfo)
		case *apiv1.StateLockError:
			return merry.Errorf("lock already acquired by %s at %s, to force unlock run:\nok force-unlock --env %s %s",
				c.env, r.Owner, r.CreatedAt.AsTime(), r.LockInfo)
		case *apiv1.ValidationError:
			return fileutil.YAMLError(yamlContext.Prefix+"."+r.Path, r.Message, yamlContext.Data)
		}
	}

	if st.Code() == codes.Unknown {
		err = merry.New(st.Message())
	}

	return c.newPluginError(msg, err)
}

func (c *Client) mapError(msg string, err error) error {
	return c.mapErrorWithContext(msg, err, &c.yamlContext)
}

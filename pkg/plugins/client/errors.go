package client

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/fileutil"
	"github.com/outblocks/outblocks-cli/internal/util"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"github.com/outblocks/outblocks-plugin-go/types"
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

	if st.Code() == codes.FailedPrecondition && st.Message() == types.LockErrorMessage {
		var locks, owners []string

		ownersMap := make(map[string]time.Time)

		for _, det := range st.Details() {
			if r, ok := det.(*apiv1.LockError); ok {
				if t, ok := ownersMap[r.Owner]; !ok || t.After(r.CreatedAt.AsTime()) {
					ownersMap[r.Owner] = r.CreatedAt.AsTime()
				}

				locks = append(locks, fmt.Sprintf("%s=%s", r.LockName, r.LockInfo))
			}
		}

		for o, t := range ownersMap {
			owners = append(owners, fmt.Sprintf("%s at %s", o, t.Local()))
		}

		return merry.Errorf("some locks already acquired by:\n%s\nto force unlock run:\nok force-unlock --env %s %s",
			strings.Join(owners, ", "), c.env, strings.Join(locks, ","))
	}

	for _, det := range st.Details() {
		switch r := det.(type) {
		case *apiv1.StateLockError:
			return merry.Errorf("lock already acquired by %s at %s, to force unlock run:\nok force-unlock --env %s %s",
				r.Owner, r.CreatedAt.AsTime(), c.env, r.LockInfo)
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

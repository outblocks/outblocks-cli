package client

import (
	"context"

	"github.com/ansel1/merry/v2"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	plugin_util "github.com/outblocks/outblocks-plugin-go/util"
)

func (c *Client) GetSecret(ctx context.Context, key, typ string, props map[string]interface{}) (val string, ok bool, err error) {
	if err := c.Start(ctx); err != nil {
		return "", false, err
	}

	res, err := c.secretPlugin().GetSecret(ctx, &apiv1.GetSecretRequest{
		Key:         key,
		SecretsType: typ,
		Properties:  plugin_util.MustNewStruct(props),
	})
	if err != nil {
		return "", false, c.mapError("get secret error", merry.Wrap(err))
	}

	return res.Value, res.Specified, nil
}

func (c *Client) SetSecret(ctx context.Context, key, value, typ string, props map[string]interface{}) (changed bool, err error) {
	if err := c.Start(ctx); err != nil {
		return false, err
	}

	ret, err := c.secretPlugin().SetSecret(ctx, &apiv1.SetSecretRequest{
		Key:         key,
		Value:       value,
		SecretsType: typ,
		Properties:  plugin_util.MustNewStruct(props),
	})
	if err != nil {
		return false, c.mapError("set secret error", merry.Wrap(err))
	}

	return ret.Changed, nil
}

func (c *Client) DeleteSecret(ctx context.Context, key, typ string, props map[string]interface{}) (deleted bool, err error) {
	if err := c.Start(ctx); err != nil {
		return false, err
	}

	ret, err := c.secretPlugin().DeleteSecret(ctx, &apiv1.DeleteSecretRequest{
		Key:         key,
		SecretsType: typ,
		Properties:  plugin_util.MustNewStruct(props),
	})
	if err != nil {
		return false, c.mapError("delete secret error", merry.Wrap(err))
	}

	return ret.Deleted, nil
}

func (c *Client) GetSecrets(ctx context.Context, typ string, props map[string]interface{}) (map[string]string, error) {
	if err := c.Start(ctx); err != nil {
		return nil, err
	}

	res, err := c.secretPlugin().GetSecrets(ctx, &apiv1.GetSecretsRequest{
		SecretsType: typ,
		Properties:  plugin_util.MustNewStruct(props),
	})
	if err != nil {
		return nil, c.mapError("get secrets error", merry.Wrap(err))
	}

	return res.Values, nil
}

func (c *Client) ReplaceSecrets(ctx context.Context, values map[string]string, typ string, props map[string]interface{}) error {
	if err := c.Start(ctx); err != nil {
		return err
	}

	_, err := c.secretPlugin().ReplaceSecrets(ctx, &apiv1.ReplaceSecretsRequest{
		Values:      values,
		SecretsType: typ,
		Properties:  plugin_util.MustNewStruct(props),
	})
	if err != nil {
		return c.mapError("set secrets error", merry.Wrap(err))
	}

	return nil
}

func (c *Client) DeleteSecrets(ctx context.Context, typ string, props map[string]interface{}) error {
	if err := c.Start(ctx); err != nil {
		return err
	}

	_, err := c.secretPlugin().DeleteSecrets(ctx, &apiv1.DeleteSecretsRequest{
		SecretsType: typ,
		Properties:  plugin_util.MustNewStruct(props),
	})
	if err != nil {
		return c.mapError("delete secrets error", merry.Wrap(err))
	}

	return nil
}

//go:build itest

package infra

import (
	"context"
	"time"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/stretchr/testify/assert"
)

func AddSecretPasswordToEnv(t common.Test) {
	common.TestSetEnv(t, model.EnvKeySecretsPassword, common.SecretPassword)
}

func AssertSecretValueFromServer(it *Test, workerKey string, secretKey string, wantValue string) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelCtx()

	check := struct {
		Event struct {
			Data struct {
				V bool `json:"value"`
			} `json:"data"`
		} `json:"genericEvent"`
	}{}

	it.NewHttpRequestWithContext(ctx).
		WithAccessToken().
		WithJsonData(map[string]any{
			"code":   "export default async (context) => ({ 'value': context.secrets.get('" + secretKey + "') === '" + wantValue + "' })",
			"action": "GENERIC_EVENT",
			"data":   map[string]any{},
		}).
		Post("/worker/api/v1/test/" + workerKey).
		Do().
		IsOk().
		AsObject(&check)

	assert.Truef(it, check.Event.Data.V, "Value mismatch for secret '%s'", secretKey)
}

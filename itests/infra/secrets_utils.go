//go:build itest

package infra

import (
	"context"
	"os"
	"time"

	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secretPassword = "P@ssw0rd!"

func MustEncryptSecret(t require.TestingT, secretValue string) string {
	encryptedValue, err := model.EncryptSecret(secretPassword, secretValue)
	require.NoError(t, err)
	return encryptedValue
}

func AddSecretPasswordToEnv(t cliTestingT) {
	err := os.Setenv(model.EnvKeySecretsPassword, secretPassword)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Unsetenv(model.EnvKeySecretsPassword)
	})
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

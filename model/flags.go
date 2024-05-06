package model

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jfrog/jfrog-client-go/utils/log"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

const (
	FlagForce            = "force"
	FlagNoTest           = "no-test"
	FlagEdit             = "edit"
	FlagNoSecrets        = "no-secrets"
	FlagJsonOutput       = "json"
	FlagTimeout          = "timeout-ms"
	defaultTimeoutMillis = 5000
)

var (
	EnvKeyServerUrl       = "JFROG_WORKER_CLI_DEV_SERVER_URL"
	EnvKeyAccessToken     = "JFROG_WORKER_CLI_DEV_ACCESS_TOKEN"
	EnvKeySecretsPassword = "JFROG_WORKER_CLI_DEV_SECRETS_PASSWORD"
	EnvKeyAddSecretValue  = "JFROG_WORKER_CLI_DEV_ADD_SECRET_VALUE"
)

type intFlagProvider interface {
	IsFlagSet(name string) bool
	GetIntFlagValue(name string) (int, error)
}

func GetJsonOutputFlag(description ...string) components.BoolFlag {
	f := components.NewBoolFlag(FlagJsonOutput, "Whether to use a json output.", components.WithBoolDefaultValue(false))
	if len(description) > 0 && description[0] != "" {
		f.Description = description[0]
	}
	return f
}

func GetTimeoutFlag() components.StringFlag {
	return components.NewStringFlag(FlagTimeout, "The request timeout in milliseconds", components.WithIntDefaultValue(defaultTimeoutMillis))
}

func GetNoSecretsFlag(description ...string) components.BoolFlag {
	f := components.NewBoolFlag(FlagNoSecrets, "Do not use registered secrets.", components.WithBoolDefaultValue(false))
	if len(description) > 0 && description[0] != "" {
		f.Description = description[0]
	}
	return f
}

func GetNoTestFlag(description ...string) components.BoolFlag {
	f := components.NewBoolFlag(FlagNoTest, "Do not generate tests.", components.WithBoolDefaultValue(false))
	if len(description) > 0 && description[0] != "" {
		f.Description = description[0]
	}
	return f
}

func GetWorkerKeyArgument() components.Argument {
	return components.Argument{
		Name:        "worker-key",
		Optional:    true,
		Description: "The worker key. If not provided it will be read from the `manifest.json` in the current directory.",
	}
}

func GetJsonPayloadArgument() components.Argument {
	return components.Argument{
		Name:        "json-payload",
		Description: "The json payload expected by the worker.\n\t\tUse '-' to read from standard input.\n\t\tUse '@<file-path>' to read from a file located at <file-path>.",
	}
}

func GetTimeoutParameter(c intFlagProvider) (time.Duration, error) {
	if !c.IsFlagSet(FlagTimeout) {
		return defaultTimeoutMillis * time.Millisecond, nil
	}
	value, err := c.GetIntFlagValue(FlagTimeout)
	if err != nil {
		log.Debug(fmt.Sprintf("Invalid timeout: %+v", err))
		return 0, errors.New("invalid timeout provided")
	}
	return time.Duration(value) * time.Millisecond, nil
}

func GetServerDetails(c *components.Context) (*config.ServerDetails, error) {
	serverUrlFromEnv, envHasServerUrl := os.LookupEnv(EnvKeyServerUrl)
	accessTokenFromEnv, envHasAccessToken := os.LookupEnv(EnvKeyAccessToken)

	if envHasServerUrl && envHasAccessToken {
		return &config.ServerDetails{Url: serverUrlFromEnv, AccessToken: accessTokenFromEnv}, nil
	}

	return plugins_common.GetServerDetails(c)
}

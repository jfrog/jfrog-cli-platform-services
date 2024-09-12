package commands

import (
	"encoding/json"
	"net/http"

	"github.com/jfrog/jfrog-client-go/utils/log"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func GetExecuteCommand() components.Command {
	return components.Command{
		Name:        "execute",
		Description: "Execute a GENERIC_EVENT worker",
		Aliases:     []string{"exec", "e"},
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			model.GetTimeoutFlag(),
			model.GetProjectKeyFlag(),
		},
		Arguments: []components.Argument{
			model.GetWorkerKeyArgument(),
			model.GetJsonPayloadArgument(),
		},
		Action: runExecuteCommand,
	}
}

func runExecuteCommand(c *components.Context) error {
	workerKey, projectKey, err := extractProjectAndKeyFromCommandContext(c, c.Arguments, 1, true)
	if err != nil {
		return err
	}

	if workerKey == "" && len(c.Arguments) > 0 {
		log.Info("No worker name provided, it will be taken from the manifest. Last argument is considered as a json payload.")
	}

	server, err := model.GetServerDetails(c)
	if err != nil {
		return err
	}

	inputReader := &cmdInputReader{c}

	data, err := inputReader.readData()
	if err != nil {
		return err
	}

	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	var queryParams map[string]string
	if projectKey != "" {
		queryParams = map[string]string{"projectKey": projectKey}
	}

	return callWorkerApiWithOutput(c, server.GetUrl(), server.GetAccessToken(), http.MethodPost, body, http.StatusOK, queryParams, "execute", workerKey)
}

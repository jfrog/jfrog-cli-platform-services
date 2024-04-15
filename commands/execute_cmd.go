package commands

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jfrog/jfrog-client-go/utils/log"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"

	"github.com/jfrog/workers-cli/model"
)

func GetExecuteCommand() components.Command {
	return components.Command{
		Name:        "execute",
		Description: "Execute a GENERIC_EVENT worker",
		Aliases:     []string{"exec", "e"},
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			model.GetTimeoutFlag(),
		},
		Arguments: []components.Argument{
			model.GetWorkerKeyArgument(),
			model.GetJsonPayloadArgument(),
		},
		Action: func(c *components.Context) error {
			var workerKey string

			if len(c.Arguments) > 1 {
				workerKey = c.Arguments[0]
			} else if len(c.Arguments) > 0 {
				log.Info("No worker name provided, it will be taken from the manifest. Last argument is considered as a json payload.")
			}

			if workerKey == "" {
				manifest, err := model.ReadManifest()
				if err != nil {
					return err
				}

				if err = manifest.Validate(); err != nil {
					return err
				}

				if manifest.Action != "GENERIC_EVENT" {
					return fmt.Errorf("only the GENERIC_EVENT actions are executable. Got %s", manifest.Action)
				}

				workerKey = manifest.Name
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

			return callWorkerApiWithOutput(c, server.GetUrl(), server.GetAccessToken(), http.MethodPost, body, http.StatusOK, "execute", workerKey)
		},
	}
}

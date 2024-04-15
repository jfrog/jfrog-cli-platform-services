package commands

import (
	"fmt"
	"net/http"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/workers-cli/model"
)

func GetRemoveCommand() components.Command {
	return components.Command{
		Name:        "undeploy",
		Description: "Undeploy a worker",
		Aliases:     []string{"rm"},
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			model.GetTimeoutFlag(),
		},
		Arguments: []components.Argument{
			model.GetWorkerKeyArgument(),
		},
		Action: func(c *components.Context) error {
			var workerKey string

			if len(c.Arguments) > 0 {
				workerKey = c.Arguments[0]
			}

			if workerKey == "" {
				manifest, err := model.ReadManifest()
				if err != nil {
					return err
				}

				if err = manifest.Validate(); err != nil {
					return err
				}

				workerKey = manifest.Name
			}

			server, err := model.GetServerDetails(c)
			if err != nil {
				return err
			}

			log.Info(fmt.Sprintf("Removing worker '%s' ...", workerKey))

			err = callWorkerApiSilent(c, server.GetUrl(), server.GetAccessToken(), http.MethodDelete, nil, http.StatusNoContent, "workers", workerKey)
			if err == nil {
				log.Info(fmt.Sprintf("Worker '%s' removed", workerKey))
			}

			return err
		},
	}
}

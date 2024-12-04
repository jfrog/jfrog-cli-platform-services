package commands

import (
	"fmt"
	"net/http"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-cli-platform-services/model"
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
		Action: runRemoveCommand,
	}
}

func runRemoveCommand(c *components.Context) error {
	workerKey, _, err := common.ExtractProjectAndKeyFromCommandContext(c, c.Arguments, 0, false)
	if err != nil {
		return err
	}

	server, err := model.GetServerDetails(c)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Removing worker '%s' ...", workerKey))

	err = common.CallWorkerApi(c, common.ApiCallParams{
		Method:      http.MethodDelete,
		ServerUrl:   server.GetUrl(),
		ServerToken: server.GetAccessToken(),
		OkStatuses:  []int{http.StatusNoContent},
		Path:        []string{"workers", workerKey},
	})
	if err == nil {
		log.Info(fmt.Sprintf("Worker '%s' removed", workerKey))
	}

	return err
}

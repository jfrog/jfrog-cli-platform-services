package commands

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
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
			format.GetFormatFlag(format.Json, format.Json),
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

	var responseStatus int
	var contentHandler common.APIContentHandler
	if slices.Contains(c.FlagsUsed, format.FlagName) {
		outputFormat, fmtErr := plugins_common.GetOutputFormat(c)
		if fmtErr != nil {
			return fmtErr
		}
		if outputFormat != format.Json {
			return fmt.Errorf("unsupported format '%s' for worker undeploy. Only json is supported", outputFormat)
		}
		contentHandler = func(body []byte) error {
			return common.PrintJSONOrStatus(responseStatus, body)
		}
	}

	log.Info(fmt.Sprintf("Removing worker '%s' ...", workerKey))

	err = common.CallWorkerAPI(c, common.APICallParams{
		Method:        http.MethodDelete,
		ServerURL:     server.GetUrl(),
		ServerToken:   server.GetAccessToken(),
		OkStatuses:    []int{http.StatusNoContent},
		Path:          []string{"workers", workerKey},
		OnContent:     contentHandler,
		CaptureStatus: &responseStatus,
	})
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Worker '%s' removed", workerKey))

	return nil
}

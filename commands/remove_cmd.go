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
		Name:             "undeploy",
		Description:      "Undeploy a worker",
		AIDescription: `Delete a deployed worker from the JFrog Platform. The worker stops handling events immediately; its source, secrets, and schedule are removed server-side. Local files (manifest.json, worker.ts) are not touched.

When to use:
- Decommissioning a worker that is no longer needed.
- Cleaning up test workers from a non-production environment.
- Removing a misconfigured worker before redeploying.

Prerequisites:
- Configured server (jf c add or jf login) with delete permission on the worker.
- If the worker is namespaced under a project, run from a directory whose manifest.json declares the project, or pass the worker key directly.

Common patterns:
  $ jf worker undeploy my-worker
  $ jf worker undeploy           # worker name taken from manifest.json
  $ jf worker undeploy my-worker --format json

Gotchas:
- This operation is irreversible from the server; redeploy with 'jf worker deploy' to restore.
- Execution history is retained server-side and remains visible via 'jf worker execution-history' for a configured retention period.
- The command exits cleanly even if the worker did not exist when run with --format json (the response body is empty/204).

Related: jf worker deploy, jf worker list`,
		Aliases:          []string{"rm"},
		SupportedFormats: []format.OutputFormat{format.Json},
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

	var responseStatus int
	var contentHandler common.APIContentHandler
	if slices.Contains(c.FlagsUsed, format.FlagName) {
		if _, fmtErr := c.GetOutputFormat(); fmtErr != nil {
			return fmtErr
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

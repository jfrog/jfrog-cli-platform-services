package commands

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	"github.com/jfrog/jfrog-client-go/utils/log"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func GetExecuteCommand() components.Command {
	return components.Command{
		Name:        "execute",
		Description: "Execute a GENERIC_EVENT worker",
		AIDescription: `Invoke a deployed GENERIC_EVENT worker on the server with a JSON payload. Unlike 'test-run', this runs the published version of the worker (including its server-side secrets) and is the supported way to trigger a worker on demand.

When to use:
- Calling a GENERIC_EVENT worker from a script or CI job.
- Smoke-testing a freshly deployed worker against production state.
- Reproducing an issue using the exact deployed code.

Prerequisites:
- The worker must already be deployed (see 'jf worker deploy') and of action type GENERIC_EVENT.
- Configured server (jf c add or jf login) with execute permission on the worker.
- If the worker is namespaced under a project, pass --project or run inside a directory whose manifest.json declares it.

Common patterns:
  $ jf worker execute my-worker '{"hello":"world"}'
  $ jf worker execute my-worker @./payload.json
  $ jf worker execute @- < payload.json    # worker name read from manifest.json
  $ jf worker execute my-worker '{}' --project my-project
  $ jf worker execute my-worker '{}' --format table

Gotchas:
- Only GENERIC_EVENT workers can be triggered with this command; event-driven workers (BEFORE_UPLOAD, etc.) fire when the underlying event occurs.
- If you omit the worker name, the name is read from manifest.json and the last argument is treated as the payload.
- Use '@file' or '@-' to load the payload from a file or stdin instead of inlining JSON.

Related: jf worker deploy, jf worker test-run, jf worker execution-history`,
		Aliases:          []string{"exec", "e"},
		SupportedFormats: []format.OutputFormat{format.Json, format.Table},
		DefaultFormat:    format.Json,
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			model.GetTimeoutFlag(),
			model.GetProjectKeyFlag(),
		},
		Arguments: []components.Argument{
			model.GetWorkerKeyArgument(),
			model.GetJSONPayloadArgument(),
		},
		Action: runExecuteCommand,
	}
}

func runExecuteCommand(c *components.Context) error {
	outputFormat, err := c.GetOutputFormat()
	if err != nil {
		return err
	}

	var contentHandler func([]byte) error
	switch outputFormat {
	case format.Json:
		contentHandler = common.PrintJSON
	case format.Table:
		contentHandler = printExecuteResponseAsTable
	}

	workerKey, projectKey, err := common.ExtractProjectAndKeyFromCommandContext(c, c.Arguments, 1, true)
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

	inputReader := common.NewInputReader(c)

	data, err := inputReader.ReadData()
	if err != nil {
		return err
	}

	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return common.CallWorkerAPI(c, common.APICallParams{
		Method:      http.MethodPost,
		ServerURL:   server.GetUrl(),
		ServerToken: server.GetAccessToken(),
		OkStatuses:  []int{http.StatusOK},
		Body:        body,
		ProjectKey:  projectKey,
		Path:        []string{"execute", workerKey},
		OnContent:   contentHandler,
	})
}

func printExecuteResponseAsTable(responseBytes []byte) error {
	var data map[string]any
	if err := json.Unmarshal(responseBytes, &data); err != nil {
		return common.PrintJSON(responseBytes)
	}

	writer := common.NewCsvWriter()
	for k, v := range data {
		if err := writer.Write([]string{k, fmt.Sprint(v)}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

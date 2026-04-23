package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-platform-services/commands/common"
	"github.com/jfrog/jfrog-cli-platform-services/model"
)

type executionHistoryResultEntry struct {
	Result string `json:"result"`
	Logs   string `json:"logs"`
}

type executionHistoryEntry struct {
	Start   time.Time                   `json:"start"`
	End     time.Time                   `json:"end"`
	TestRun bool                        `json:"testRun"`
	Entries executionHistoryResultEntry `json:"entries"`
}

func GetShowExecutionHistoryCommand() components.Command {
	return components.Command{
		Name:        "execution-history",
		Description: "Show a worker execution history.",
		Aliases:     []string{"exec-hist", "eh"},
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			format.GetFormatFlag(format.Json, format.Json, format.Table),
			model.GetTimeoutFlag(),
			model.GetProjectKeyFlag(),
			components.NewBoolFlag(
				"with-test-runs",
				"Whether to include test-runs entries.",
				components.WithBoolDefaultValue(false),
			),
		},
		Arguments: []components.Argument{
			model.GetWorkerKeyArgument(),
		},
		Action: func(c *components.Context) error {
			workerKey, projectKey, err := common.ExtractProjectAndKeyFromCommandContext(c, c.Arguments, 1, false)
			if err != nil {
				return err
			}

			server, err := model.GetServerDetails(c)
			if err != nil {
				return err
			}

			query := map[string]string{
				"workerKey": workerKey,
			}

			if c.GetBoolFlagValue("with-test-runs") {
				query["showTestRun"] = "true"
			}

			outputFormat, err := plugins_common.GetOutputFormat(c)
			if err != nil {
				return err
			}

			var contentHandler func([]byte) error
			switch outputFormat {
			case format.Json:
				contentHandler = common.PrintJSON
			case format.Table:
				contentHandler = printExecutionHistoryTable
			default:
				return fmt.Errorf("unsupported format '%s'. Accepted values: json, table", outputFormat)
			}

			return common.CallWorkerAPI(c, common.APICallParams{
				Method:      http.MethodGet,
				ServerURL:   server.GetUrl(),
				ServerToken: server.GetAccessToken(),
				OkStatuses:  []int{http.StatusOK},
				ProjectKey:  projectKey,
				Query:       query,
				Path:        []string{"execution_history"},
				OnContent:   contentHandler,
			})
		},
	}
}

func printExecutionHistoryTable(responseBytes []byte) error {
	var entries []executionHistoryEntry
	if err := json.Unmarshal(responseBytes, &entries); err != nil {
		return err
	}

	writer := common.NewCsvWriter()
	for _, entry := range entries {
		if err := writer.Write([]string{
			entry.Start.Format(time.RFC3339),
			entry.End.Format(time.RFC3339),
			fmt.Sprint(entry.TestRun),
			entry.Entries.Result,
			entry.Entries.Logs,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

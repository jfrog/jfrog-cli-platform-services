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

type executionHistoryEntry struct {
	WorkerKey        string `json:"workerKey"`
	WorkerType       string `json:"workerType"`
	WorkerProjectKey string `json:"workerProjectKey"`
	ExecutionStatus  string `json:"executionStatus"`
	StartTimeMillis  int64  `json:"startTimeMillis"`
	EndTimeMillis    int64  `json:"endTimeMillis"`
	TriggeredBy      string `json:"triggeredBy"`
	TestRun          bool   `json:"testRun"`
	ExecutedVersion  string `json:"executedVersion"`
	TraceID          string `json:"traceId"`
}

func GetShowExecutionHistoryCommand() components.Command {
	return components.Command{
		Name:             "execution-history",
		Description:      "Show a worker execution history.",
		Aliases:          []string{"exec-hist", "eh"},
		SupportedFormats: []format.OutputFormat{format.Json, format.Table},
		DefaultFormat:    format.Json,
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
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
			outputFormat, err := c.GetOutputFormat()
			if err != nil {
				return err
			}

			var contentHandler func([]byte) error
			switch outputFormat {
			case format.Json:
				contentHandler = common.PrintJSON
			case format.Table:
				contentHandler = printExecutionHistoryTable
			}

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

	if err := writer.Write([]string{
		"Worker Key",
		"Worker Type",
		"Project Key",
		"Status",
		"Started At",
		"Ended At",
		"Triggered By",
		"Test Run",
		"Executed Version",
		"Trace ID",
	}); err != nil {
		return err
	}

	for _, entry := range entries {
		startedAt := time.UnixMilli(entry.StartTimeMillis).UTC().Format(time.RFC3339)
		endedAt := time.UnixMilli(entry.EndTimeMillis).UTC().Format(time.RFC3339)
		if err := writer.Write([]string{
			entry.WorkerKey,
			entry.WorkerType,
			entry.WorkerProjectKey,
			entry.ExecutionStatus,
			startedAt,
			endedAt,
			entry.TriggeredBy,
			fmt.Sprint(entry.TestRun),
			entry.ExecutedVersion,
			entry.TraceID,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

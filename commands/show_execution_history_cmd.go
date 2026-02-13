package commands

import (
	"net/http"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func GetShowExecutionHistoryCommand() components.Command {
	return components.Command{
		Name:        "execution-history",
		Description: "Show a worker execution history.",
		Aliases:     []string{"exec-hist", "eh"},
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
				OnContent:   common.PrintJSON,
			})
		},
	}
}

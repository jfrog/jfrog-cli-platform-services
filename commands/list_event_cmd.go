package commands

import (
	"net/http"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func GetListEventsCommand() components.Command {
	return components.Command{
		Name:        "list-event",
		Description: "List available events.",
		Aliases:     []string{"le"},
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			model.GetTimeoutFlag(),
			model.GetProjectKeyFlag(),
		},
		Action: func(c *components.Context) error {
			server, err := model.GetServerDetails(c)
			if err != nil {
				return err
			}

			projectKey := c.GetStringFlagValue(model.FlagProjectKey)

			var queryParams map[string]string
			if projectKey != "" {
				queryParams = map[string]string{"projectKey": projectKey}
			}

			return callWorkerApiWithOutput(c, server.GetUrl(), server.GetAccessToken(), http.MethodGet, nil, http.StatusOK, queryParams, "actions")
		},
	}
}

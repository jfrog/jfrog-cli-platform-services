package commands

import (
	"strings"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

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

			actionsMeta, err := common.FetchActions(c, server.Url, server.AccessToken, projectKey)
			if err != nil {
				return err
			}

			var actions []string
			for _, md := range actionsMeta {
				actions = append(actions, md.Action.Name)
			}

			return common.Print("%s", strings.Join(actions, ", "))
		},
	}
}

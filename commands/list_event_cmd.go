package commands

import (
	"fmt"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"
	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func GetListEventsCommand() components.Command {
	return components.Command{
		Name:        "list-event",
		Description: "List available events.",
		Aliases:     []string{"le"},
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			format.GetFormatFlag(format.Table, format.Json, format.Table),
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

			outputFormat, err := plugins_common.GetOutputFormat(c)
			if err != nil {
				return err
			}

			switch outputFormat {
			case format.Json:
				return common.PrintJSONValue(actionsMeta)
			case format.Table:
				return printListEventTable(actionsMeta)
			default:
				return fmt.Errorf("unsupported format '%s'. Accepted values: json, table", outputFormat)
			}
		},
	}
}

func printListEventTable(actionsMeta common.ActionsMetadata) error {
	return common.Print("%s", strings.Join(actionsMeta.ActionsNames(), ", "))
}

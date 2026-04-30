package commands

import (
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"
	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func GetListEventsCommand() components.Command {
	return components.Command{
		Name:             "list-event",
		Description:      "List available events.",
		Aliases:          []string{"le"},
		SupportedFormats: []format.OutputFormat{format.Json, format.Table},
		DefaultFormat:    format.None,
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

			outputFormat, err := c.GetOutputFormat()
			if err != nil {
				return err
			}

			actionsMeta, err := common.FetchActions(c, server.Url, server.AccessToken, projectKey)
			if err != nil {
				return err
			}

			if outputFormat == format.None {
				// Old behavior: no --format flag
				return common.Print("%s", strings.Join(actionsMeta.ActionsNames(), ", "))
			}
			switch outputFormat {
			case format.Json:
				return common.PrintJSONValue(actionsMeta)
			default:
				return printListEventTable(actionsMeta)
			}
		},
	}
}

func printListEventTable(actionsMeta common.ActionsMetadata) error {
	writer := common.NewCsvWriter()
	if err := writer.Write([]string{"NAME", "APPLICATION", "DESCRIPTION"}); err != nil {
		return err
	}
	for _, action := range actionsMeta {
		if err := writer.Write([]string{
			action.Action.Name,
			action.Action.Application,
			action.Description,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

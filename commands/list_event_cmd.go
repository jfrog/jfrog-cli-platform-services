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
		AIDescription: `List the action / event types supported by the target server. Use this to discover the valid 'action' values for 'jf worker init' and 'jf worker list'.

When to use:
- Looking up the exact name of an action (e.g. BEFORE_UPLOAD, GENERIC_EVENT, SCHEDULED_EVENT) before scaffolding a worker.
- Auditing which applications expose events on a given platform.
- Inspecting action descriptions and metadata before designing a worker.

Prerequisites:
- Configured server (jf c add or jf login).
- For project-scoped action sets, pass --project.

Common patterns:
  $ jf worker list-event                       # comma-separated names (legacy output)
  $ jf worker list-event --format table        # NAME,APPLICATION,DESCRIPTION CSV
  $ jf worker list-event --format json         # full action metadata, including TypeScript types
  $ jf worker list-event --project my-project --format table

Gotchas:
- Without --format, the output is a plain comma-separated list (kept for backwards compatibility); pass --format table or --format json for structured output.
- The set of actions depends on the server's installed applications and on the --project scope.

Related: jf worker init, jf worker list`,
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

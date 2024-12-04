package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-platform-services/model"
)

type getAllResponse struct {
	Workers []*model.WorkerDetails `json:"workers"`
}

func GetListCommand() components.Command {
	return components.Command{
		Name:        "list",
		Description: "List workers. The default output is a CSV format with columns <name>,<action>,<description>,<enabled>.",
		Aliases:     []string{"ls"},
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			model.GetJsonOutputFlag("Use JSON instead of CSV as output"),
			model.GetTimeoutFlag(),
			model.GetProjectKeyFlag(),
		},
		Arguments: []components.Argument{
			{
				Name:        "action",
				Description: "Only show workers of this type.\n\t\tUse `jf worker list-event` to see all available actions.",
				Optional:    true,
			},
		},
		Action: func(c *components.Context) error {
			server, err := model.GetServerDetails(c)
			if err != nil {
				return err
			}
			return runListCommand(c, server.GetUrl(), server.GetAccessToken())
		},
	}
}

func runListCommand(ctx *components.Context, serverUrl string, token string) error {
	params := make(map[string]string)

	var action string

	if len(ctx.Arguments) > 0 {
		action = strings.TrimSpace(ctx.Arguments[0])
	}
	if action != "" {
		params["action"] = action
	}

	contentHandler := printWorkerDetailsAsCsv
	if ctx.GetBoolFlagValue(model.FlagJsonOutput) {
		contentHandler = common.PrintJson
	}

	return common.CallWorkerApi(ctx, common.ApiCallParams{
		Method:      http.MethodGet,
		ServerUrl:   serverUrl,
		ServerToken: token,
		Query:       params,
		OkStatuses:  []int{http.StatusOK},
		Path:        []string{"workers"},
		ProjectKey:  ctx.GetStringFlagValue(model.FlagProjectKey),
		OnContent:   contentHandler,
	})
}

func printWorkerDetailsAsCsv(responseBytes []byte) error {
	var err error
	allWorkers := getAllResponse{}

	err = json.Unmarshal(responseBytes, &allWorkers)
	if err != nil {
		return nil
	}

	writer := common.NewCsvWriter()

	slices.SortFunc(allWorkers.Workers, func(a, b *model.WorkerDetails) int {
		return strings.Compare(a.Key, b.Key)
	})

	for _, wk := range allWorkers.Workers {
		err = writer.Write([]string{
			wk.Key, wk.Action, wk.Description, fmt.Sprint(wk.Enabled),
		})
		if err != nil {
			return err
		}
	}

	writer.Flush()

	return writer.Error()
}

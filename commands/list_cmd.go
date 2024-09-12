package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils/log"

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
				Description: fmt.Sprintf("Only show workers of this type.\n\t\tShould be one of (%s).", strings.Join(strings.Split(model.ActionNames(), "|"), ", ")),
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
	api := "workers"
	params := make(map[string]string)

	if len(ctx.Arguments) > 0 {
		params["action"] = ctx.Arguments[0]
	}

	projectKey := ctx.GetStringFlagValue(model.FlagProjectKey)
	if projectKey != "" {
		params["projectKey"] = projectKey
	}

	res, discardReq, err := callWorkerApi(ctx, serverUrl, token, http.MethodGet, nil, params, api)
	if discardReq != nil {
		defer discardReq()
	}
	if err != nil {
		return err
	}

	if ctx.GetBoolFlagValue(model.FlagJsonOutput) {
		return outputApiResponse(res, http.StatusOK)
	}

	return formatListResponseAsCsv(res, http.StatusOK)
}

func formatListResponseAsCsv(res *http.Response, okStatus int) error {
	return processApiResponse(res, func(responseBytes []byte, statusCode int) error {
		var err error

		if res.StatusCode != okStatus {
			err = fmt.Errorf("command failed with status %d", res.StatusCode)
		}

		if err == nil {
			allWorkers := getAllResponse{}

			err = json.Unmarshal(responseBytes, &allWorkers)
			if err != nil {
				return nil
			}

			writer := csv.NewWriter(cliOut)

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
		} else if len(responseBytes) > 0 {
			// We will report the previous error, but we still want to display the response body
			if _, writeErr := cliOut.Write(prettifyJson(responseBytes)); writeErr != nil {
				log.Debug(fmt.Sprintf("Write error: %+v", writeErr))
			}
		}

		return err
	})
}

package commands

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	"github.com/jfrog/jfrog-client-go/utils/log"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

type dryRunHandler struct {
	ctx *components.Context
}

type dryRunRequest struct {
	Code          string          `json:"code"`
	Action        string          `json:"action"`
	StagedSecrets []*model.Secret `json:"stagedSecrets,omitempty"`
	Data          map[string]any  `json:"data"`
}

func GetDryRunCommand() components.Command {
	return components.Command{
		Name:        "test-run",
		Description: "Dry run a worker",
		Aliases:     []string{"dry-run", "dr", "tr"},
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			model.GetTimeoutFlag(),
			model.GetNoSecretsFlag(),
		},
		Arguments: []components.Argument{
			model.GetJsonPayloadArgument(),
		},
		Action: func(c *components.Context) error {
			h := &dryRunHandler{c}

			manifest, err := common.ReadManifest()
			if err != nil {
				return err
			}

			server, err := model.GetServerDetails(c)
			if err != nil {
				return err
			}

			actionsMeta, err := common.FetchActions(c, server.GetUrl(), server.GetAccessToken(), manifest.ProjectKey)
			if err != nil {
				return err
			}

			if err = common.ValidateManifest(manifest, actionsMeta); err != nil {
				return err
			}

			inputReader := common.NewInputReader(c)

			data, err := inputReader.ReadData()
			if err != nil {
				return err
			}

			if !c.GetBoolFlagValue(model.FlagNoSecrets) {
				if err = common.DecryptManifestSecrets(manifest); err != nil {
					return err
				}
			}

			return h.run(manifest, server.GetUrl(), server.GetAccessToken(), data)
		},
	}
}

func (c *dryRunHandler) run(manifest *model.Manifest, serverUrl string, token string, data map[string]any) error {
	body, err := c.preparePayload(manifest, serverUrl, token, data)
	if err != nil {
		return err
	}
	return common.CallWorkerApi(c.ctx, common.ApiCallParams{
		Method:      http.MethodPost,
		ServerUrl:   serverUrl,
		ServerToken: token,
		Body:        body,
		ProjectKey:  manifest.ProjectKey,
		Query: map[string]string{
			"debug": fmt.Sprint(manifest.Debug),
		},
		OkStatuses: []int{http.StatusOK},
		Path:       []string{"test", manifest.Name},
		OnContent:  common.PrintJson,
	})
}

func (c *dryRunHandler) preparePayload(manifest *model.Manifest, serverUrl string, token string, data map[string]any) ([]byte, error) {
	payload := &dryRunRequest{Action: manifest.Action, Data: data}

	var err error

	payload.Code, err = common.ReadSourceCode(manifest)
	if err != nil {
		return nil, err
	}
	payload.Code = common.CleanImports(payload.Code)

	existingWorker, err := common.FetchWorkerDetails(c.ctx, serverUrl, token, manifest.Name, manifest.ProjectKey)
	if err != nil {
		log.Warn(err.Error())
	}

	if !c.ctx.GetBoolFlagValue(model.FlagNoSecrets) {
		payload.StagedSecrets = common.PrepareSecretsUpdate(manifest, existingWorker)
	}

	return json.Marshal(&payload)
}

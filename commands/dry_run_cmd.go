package commands

import (
	"encoding/json"
	"net/http"

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
		Name:        "dry-run",
		Description: "Dry run a worker",
		Aliases:     []string{"dr"},
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

			manifest, err := model.ReadManifest()
			if err != nil {
				return err
			}

			if err = manifest.Validate(); err != nil {
				return err
			}

			server, err := model.GetServerDetails(c)
			if err != nil {
				return err
			}

			inputReader := &cmdInputReader{c}

			data, err := inputReader.readData()
			if err != nil {
				return err
			}

			if !c.GetBoolFlagValue(model.FlagNoSecrets) {
				if err = manifest.DecryptSecrets(); err != nil {
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
	return callWorkerApiWithOutput(c.ctx, serverUrl, token, http.MethodPost, body, http.StatusOK, "test", manifest.Name)
}

func (c *dryRunHandler) preparePayload(manifest *model.Manifest, serverUrl string, token string, data map[string]any) ([]byte, error) {
	payload := &dryRunRequest{Action: manifest.Action, Data: data}

	var err error

	payload.Code, err = manifest.ReadSourceCode()
	if err != nil {
		return nil, err
	}
	payload.Code = cleanImports(payload.Code)

	existingWorker, err := fetchWorkerDetails(c.ctx, serverUrl, token, manifest.Name)
	if err != nil {
		log.Warn(err.Error())
	}

	if !c.ctx.GetBoolFlagValue(model.FlagNoSecrets) {
		payload.StagedSecrets = prepareSecretsUpdate(manifest, existingWorker)
	}

	return json.Marshal(&payload)
}

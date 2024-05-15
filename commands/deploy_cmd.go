package commands

import (
	"encoding/json"
	"fmt"
	"net/http"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

type deployRequest struct {
	Key            string               `json:"key"`
	Description    string               `json:"description"`
	Enabled        bool                 `json:"enabled"`
	SourceCode     string               `json:"sourceCode"`
	Action         string               `json:"action"`
	FilterCriteria model.FilterCriteria `json:"filterCriteria,omitempty"`
	Secrets        []*model.Secret      `json:"secrets"`
}

func GetDeployCommand() components.Command {
	return components.Command{
		Name:        "deploy",
		Description: "Deploy a worker",
		Aliases:     []string{"d"},
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			model.GetTimeoutFlag(),
			model.GetNoSecretsFlag(),
		},
		Action: func(c *components.Context) error {
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

			if !c.GetBoolFlagValue(model.FlagNoSecrets) {
				if err = manifest.DecryptSecrets(); err != nil {
					return err
				}
			}

			return runDeployCommand(c, manifest, server.GetUrl(), server.GetAccessToken())
		},
	}
}

func runDeployCommand(ctx *components.Context, manifest *model.Manifest, serverUrl string, token string) error {
	existingWorker, err := fetchWorkerDetails(ctx, serverUrl, token, manifest.Name)
	if err != nil {
		return err
	}

	body, err := prepareDeployRequest(ctx, manifest, existingWorker)
	if err != nil {
		return err
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	if existingWorker == nil {
		log.Info(fmt.Sprintf("Deploying worker '%s'", manifest.Name))
		err = callWorkerApiWithOutput(ctx, serverUrl, token, http.MethodPost, bodyBytes, http.StatusCreated, "workers")
		if err == nil {
			log.Info(fmt.Sprintf("Worker '%s' deployed", manifest.Name))
		}
		return err
	}

	log.Info(fmt.Sprintf("Updating worker '%s'", manifest.Name))
	err = callWorkerApiWithOutput(ctx, serverUrl, token, http.MethodPut, bodyBytes, http.StatusNoContent, "workers")
	if err == nil {
		log.Info(fmt.Sprintf("Worker '%s' updated", manifest.Name))
	}

	return err
}

func prepareDeployRequest(ctx *components.Context, manifest *model.Manifest, existingWorker *model.WorkerDetails) (*deployRequest, error) {
	sourceCode, err := manifest.ReadSourceCode()
	if err != nil {
		return nil, err
	}
	sourceCode = cleanImports(sourceCode)

	var secrets []*model.Secret

	if !ctx.GetBoolFlagValue(model.FlagNoSecrets) {
		secrets = prepareSecretsUpdate(manifest, existingWorker)
	}

	payload := &deployRequest{
		Key:            manifest.Name,
		Action:         manifest.Action,
		Description:    manifest.Description,
		Enabled:        manifest.Enabled,
		FilterCriteria: manifest.FilterCriteria,
		SourceCode:     sourceCode,
		Secrets:        secrets,
	}

	return payload, nil
}

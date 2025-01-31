package commands

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

type deployRequest struct {
	Key            string                `json:"key"`
	Description    string                `json:"description"`
	Enabled        bool                  `json:"enabled"`
	Debug          bool                  `json:"debug"`
	SourceCode     string                `json:"sourceCode"`
	Action         string                `json:"action"`
	FilterCriteria *model.FilterCriteria `json:"filterCriteria,omitempty"`
	Secrets        []*model.Secret       `json:"secrets"`
	ProjectKey     string                `json:"projectKey"`
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
			server, err := model.GetServerDetails(c)
			if err != nil {
				return err
			}

			manifest, err := common.ReadManifest()
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

			actionMeta, err := actionsMeta.FindAction(manifest.Action, manifest.Application)
			if err != nil {
				return err
			}

			if err = common.ValidateFilterCriteria(&manifest.FilterCriteria, actionMeta); err != nil {
				return err
			}

			if !c.GetBoolFlagValue(model.FlagNoSecrets) {
				if err = common.DecryptManifestSecrets(manifest); err != nil {
					return err
				}
			}

			return runDeployCommand(c, manifest, actionMeta, server.GetUrl(), server.GetAccessToken())
		},
	}
}

func runDeployCommand(ctx *components.Context, manifest *model.Manifest, actionMeta *model.ActionMetadata, serverUrl string, token string) error {
	existingWorker, err := common.FetchWorkerDetails(ctx, serverUrl, token, manifest.Name, manifest.ProjectKey)
	if err != nil {
		return err
	}

	body, err := prepareDeployRequest(ctx, manifest, actionMeta, existingWorker)
	if err != nil {
		return err
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	if existingWorker == nil {
		log.Info(fmt.Sprintf("Deploying worker '%s'", manifest.Name))
		err = common.CallWorkerApi(ctx, common.ApiCallParams{
			Method:      http.MethodPost,
			ServerUrl:   serverUrl,
			ServerToken: token,
			Body:        bodyBytes,
			OkStatuses:  []int{http.StatusCreated},
			Path:        []string{"workers"},
		})
		if err == nil {
			log.Info(fmt.Sprintf("Worker '%s' deployed", manifest.Name))
		}
		return err
	}

	log.Info(fmt.Sprintf("Updating worker '%s'", manifest.Name))
	err = common.CallWorkerApi(ctx, common.ApiCallParams{
		Method:      http.MethodPut,
		ServerUrl:   serverUrl,
		ServerToken: token,
		Body:        bodyBytes,
		OkStatuses:  []int{http.StatusNoContent},
		Path:        []string{"workers"},
	})
	if err == nil {
		log.Info(fmt.Sprintf("Worker '%s' updated", manifest.Name))
	}

	return err
}

func prepareDeployRequest(ctx *components.Context, manifest *model.Manifest, actionMeta *model.ActionMetadata, existingWorker *model.WorkerDetails) (*deployRequest, error) {
	sourceCode, err := common.ReadSourceCode(manifest)
	if err != nil {
		return nil, err
	}
	sourceCode = common.CleanImports(sourceCode)

	var secrets []*model.Secret

	if !ctx.GetBoolFlagValue(model.FlagNoSecrets) {
		secrets = common.PrepareSecretsUpdate(manifest, existingWorker)
	}

	payload := &deployRequest{
		Key:         manifest.Name,
		Action:      manifest.Action,
		Description: manifest.Description,
		Enabled:     manifest.Enabled,
		Debug:       manifest.Debug,
		SourceCode:  sourceCode,
		Secrets:     secrets,
		ProjectKey:  manifest.ProjectKey,
	}

	if actionMeta.MandatoryFilter {
		payload.FilterCriteria = &manifest.FilterCriteria
	}

	return payload, nil
}

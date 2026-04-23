package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
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
	Action         model.Action          `json:"action"`
	FilterCriteria *model.FilterCriteria `json:"filterCriteria,omitempty"`
	Secrets        []*model.Secret       `json:"secrets"`
	ProjectKey     string                `json:"projectKey"`
	Version        *model.Version        `json:"version,omitempty"`
}

func GetDeployCommand() components.Command {
	return components.Command{
		Name:        "deploy",
		Description: "Deploy a worker",
		Aliases:     []string{"d"},
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			format.GetFormatFlag(format.Json, format.Json),
			model.GetTimeoutFlag(),
			model.GetNoSecretsFlag(),
			model.GetChangesVersionFlag(),
			model.GetChangesDescriptionFlag(),
			model.GetChangesCommitShaFlag(),
			model.GetBase64Flag(),
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

			if err = common.ValidateFilterCriteria(manifest.FilterCriteria, actionMeta); err != nil {
				return err
			}

			if !c.GetBoolFlagValue(model.FlagNoSecrets) {
				if err = common.DecryptManifestSecrets(manifest); err != nil {
					return err
				}
			}

			version := &model.Version{
				Number:      c.GetStringFlagValue(model.FlagChangesVersion),
				Description: c.GetStringFlagValue(model.FlagChangesDescription),
				CommitSha:   c.GetStringFlagValue(model.FlagChangesCommitSha),
			}

			options, err := common.FetchOptions(c, server.GetUrl(), server.GetAccessToken())
			if err != nil {
				return err
			}

			if !version.IsEmpty() {
				if err = common.ValidateVersion(version, options); err != nil {
					return err
				}
			}

			var encodeSourceCodeInBase64 bool
			if options.ShouldEncodeSourceCodeInBase64 == nil {
				if c.IsFlagSet(model.FlagBase64) {
					log.Warn("The --base64 flag is not supported by this server. It will be ignored.")
				}
			} else {
				encodeSourceCodeInBase64 = *options.ShouldEncodeSourceCodeInBase64 || c.GetBoolFlagValue(model.FlagBase64)
			}
			return runDeployCommand(c, manifest, actionMeta, version, server.GetUrl(), server.GetAccessToken(), encodeSourceCodeInBase64)
		},
	}
}

func runDeployCommand(ctx *components.Context, manifest *model.Manifest, actionMeta *model.ActionMetadata, version *model.Version, serverURL string, token string, encodeSourceCodeInBase64 bool) error {
	existingWorker, err := common.FetchWorkerDetails(ctx, serverURL, token, manifest.Name, manifest.ProjectKey)
	if err != nil {
		return err
	}

	body, err := prepareDeployRequest(ctx, manifest, actionMeta, version, existingWorker, encodeSourceCodeInBase64)
	if err != nil {
		return err
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	var responseStatus int
	var contentHandler common.APIContentHandler
	if slices.Contains(ctx.FlagsUsed, format.FlagName) {
		outputFormat, fmtErr := plugins_common.GetOutputFormat(ctx)
		if fmtErr != nil {
			return fmtErr
		}
		if outputFormat != format.Json {
			return fmt.Errorf("unsupported format '%s' for worker deploy. Only json is supported", outputFormat)
		}
		contentHandler = func(body []byte) error {
			return common.PrintJSONOrStatus(responseStatus, body)
		}
	}

	if existingWorker == nil {
		log.Info(fmt.Sprintf("Deploying worker '%s'", manifest.Name))
		err = common.CallWorkerAPI(ctx, common.APICallParams{
			Method:        http.MethodPost,
			ServerURL:     serverURL,
			ServerToken:   token,
			Body:          bodyBytes,
			OkStatuses:    []int{http.StatusCreated},
			Path:          []string{"workers"},
			APIVersion:    common.APIVersionV2,
			OnContent:     contentHandler,
			CaptureStatus: &responseStatus,
		})
		if err == nil {
			log.Info(fmt.Sprintf("Worker '%s' deployed", manifest.Name))
		}
	} else {
		log.Info(fmt.Sprintf("Updating worker '%s'", manifest.Name))
		err = common.CallWorkerAPI(ctx, common.APICallParams{
			Method:        http.MethodPut,
			ServerURL:     serverURL,
			ServerToken:   token,
			Body:          bodyBytes,
			OkStatuses:    []int{http.StatusNoContent},
			Path:          []string{"workers"},
			APIVersion:    common.APIVersionV2,
			OnContent:     contentHandler,
			CaptureStatus: &responseStatus,
		})
		if err == nil {
			log.Info(fmt.Sprintf("Worker '%s' updated", manifest.Name))
		}
	}

	return err
}

func prepareDeployRequest(ctx *components.Context, manifest *model.Manifest, actionMeta *model.ActionMetadata, version *model.Version, existingWorker *model.WorkerDetails, encodeSourceCodeInBase64 bool) (*deployRequest, error) {
	sourceCode, err := common.ReadSourceCode(manifest)
	if err != nil {
		return nil, err
	}
	sourceCode = common.CleanImports(sourceCode)

	if encodeSourceCodeInBase64 {
		sourceCode = "base64:" + base64.StdEncoding.EncodeToString([]byte(sourceCode))
	}

	var secrets []*model.Secret

	if !ctx.GetBoolFlagValue(model.FlagNoSecrets) {
		secrets = common.PrepareSecretsUpdate(manifest, existingWorker)
	}
	payload := &deployRequest{
		Key:         manifest.Name,
		Action:      actionMeta.Action,
		Description: manifest.Description,
		Enabled:     manifest.Enabled,
		Debug:       manifest.Debug,
		SourceCode:  sourceCode,
		Secrets:     secrets,
		ProjectKey:  manifest.ProjectKey,
		Version:     version,
	}

	if actionMeta.MandatoryFilter {
		payload.FilterCriteria = manifest.FilterCriteria
	}
	return payload, nil
}

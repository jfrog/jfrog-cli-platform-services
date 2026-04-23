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

type deployCommandHandler struct {
	ctx                      *components.Context
	manifest                 *model.Manifest
	actionMeta               *model.ActionMetadata
	version                  *model.Version
	serverURL                string
	token                    string
	encodeSourceCodeInBase64 bool
	outputFormat             format.OutputFormat
}

func GetDeployCommand() components.Command {
	return components.Command{
		Name:             "deploy",
		Description:      "Deploy a worker",
		Aliases:          []string{"d"},
		SupportedFormats: []format.OutputFormat{format.Json},
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			model.GetTimeoutFlag(),
			model.GetNoSecretsFlag(),
			model.GetChangesVersionFlag(),
			model.GetChangesDescriptionFlag(),
			model.GetChangesCommitShaFlag(),
			model.GetBase64Flag(),
		},
		Action: func(c *components.Context) error {
			var outputFormat format.OutputFormat
			if slices.Contains(c.FlagsUsed, format.FlagName) {
				var fmtErr error
				outputFormat, fmtErr = c.GetOutputFormat()
				if fmtErr != nil {
					return fmtErr
				}
			}

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

			return (&deployCommandHandler{
				ctx:                      c,
				manifest:                 manifest,
				actionMeta:               actionMeta,
				version:                  version,
				serverURL:                server.GetUrl(),
				token:                    server.GetAccessToken(),
				encodeSourceCodeInBase64: encodeSourceCodeInBase64,
				outputFormat:             outputFormat,
			}).run()
		},
	}
}

func (h *deployCommandHandler) run() error {
	existingWorker, err := common.FetchWorkerDetails(h.ctx, h.serverURL, h.token, h.manifest.Name, h.manifest.ProjectKey)
	if err != nil {
		return err
	}

	body, err := h.prepareRequest(existingWorker)
	if err != nil {
		return err
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	var responseStatus int
	var contentHandler common.APIContentHandler
	if h.outputFormat != format.None {
		contentHandler = func(body []byte) error {
			return common.PrintJSONOrStatus(responseStatus, body)
		}
	}

	if existingWorker == nil {
		log.Info(fmt.Sprintf("Deploying worker '%s'", h.manifest.Name))
		err = common.CallWorkerAPI(h.ctx, common.APICallParams{
			Method:        http.MethodPost,
			ServerURL:     h.serverURL,
			ServerToken:   h.token,
			Body:          bodyBytes,
			OkStatuses:    []int{http.StatusCreated},
			Path:          []string{"workers"},
			APIVersion:    common.APIVersionV2,
			OnContent:     contentHandler,
			CaptureStatus: &responseStatus,
		})
		if err == nil {
			log.Info(fmt.Sprintf("Worker '%s' deployed", h.manifest.Name))
		}
	} else {
		log.Info(fmt.Sprintf("Updating worker '%s'", h.manifest.Name))
		err = common.CallWorkerAPI(h.ctx, common.APICallParams{
			Method:        http.MethodPut,
			ServerURL:     h.serverURL,
			ServerToken:   h.token,
			Body:          bodyBytes,
			OkStatuses:    []int{http.StatusNoContent},
			Path:          []string{"workers"},
			APIVersion:    common.APIVersionV2,
			OnContent:     contentHandler,
			CaptureStatus: &responseStatus,
		})
		if err == nil {
			log.Info(fmt.Sprintf("Worker '%s' updated", h.manifest.Name))
		}
	}

	return err
}

func (h *deployCommandHandler) prepareRequest(existingWorker *model.WorkerDetails) (*deployRequest, error) {
	sourceCode, err := common.ReadSourceCode(h.manifest)
	if err != nil {
		return nil, err
	}
	sourceCode = common.CleanImports(sourceCode)

	if h.encodeSourceCodeInBase64 {
		sourceCode = "base64:" + base64.StdEncoding.EncodeToString([]byte(sourceCode))
	}

	var secrets []*model.Secret
	if !h.ctx.GetBoolFlagValue(model.FlagNoSecrets) {
		secrets = common.PrepareSecretsUpdate(h.manifest, existingWorker)
	}

	payload := &deployRequest{
		Key:         h.manifest.Name,
		Action:      h.actionMeta.Action,
		Description: h.manifest.Description,
		Enabled:     h.manifest.Enabled,
		Debug:       h.manifest.Debug,
		SourceCode:  sourceCode,
		Secrets:     secrets,
		ProjectKey:  h.manifest.ProjectKey,
		Version:     h.version,
	}

	if h.actionMeta.MandatoryFilter {
		payload.FilterCriteria = h.manifest.FilterCriteria
	}
	return payload, nil
}

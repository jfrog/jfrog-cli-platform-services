package commands

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
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
		Name:             "test-run",
		Description:      "Dry run a worker",
		AIDescription: `Send the local worker source code to the platform's sandbox and execute it against a sample payload without deploying. Use this to iterate on logic before 'jf worker deploy'.

When to use:
- Validating new or modified worker.ts logic against a realistic payload.
- Reproducing a failure without touching the deployed worker.
- Smoke-testing secret handling before promoting changes.

Prerequisites:
- A valid manifest.json and worker.ts in the current directory (run 'jf worker init' first).
- Configured server (jf c add or jf login).
- The action declared in manifest.json must be supported by the target server.

Common patterns:
  $ jf worker test-run '{"repoPath":"my-repo/path/to/artifact"}'
  $ jf worker test-run @./sample-payload.json
  $ jf worker test-run @- < sample-payload.json
  $ jf worker test-run --no-secrets '{}'
  $ jf worker test-run --format table '{}'

Gotchas:
- The payload argument is required and must match what the action delivers at runtime; check types.ts for the expected shape.
- Use '@filename' to load the payload from a file and '@-' to read it from stdin.
- By default, secrets in manifest.json are decrypted and sent as staged secrets; pass --no-secrets to omit them.
- The 'debug' flag in manifest.json controls whether debug logs are returned by the sandbox.

Related: jf worker deploy, jf worker execute, jf worker init`,
		Aliases:          []string{"dry-run", "dr", "tr"},
		SupportedFormats: []format.OutputFormat{format.Json, format.Table},
		DefaultFormat:    format.Json,
		Flags: []components.Flag{
			plugins_common.GetServerIdFlag(),
			model.GetTimeoutFlag(),
			model.GetNoSecretsFlag(),
		},
		Arguments: []components.Argument{
			model.GetJSONPayloadArgument(),
		},
		Action: func(c *components.Context) error {
			outputFormat, err := c.GetOutputFormat()
			if err != nil {
				return err
			}

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

			return h.run(manifest, server.GetUrl(), server.GetAccessToken(), data, outputFormat)
		},
	}
}

func (c *dryRunHandler) run(manifest *model.Manifest, serverURL string, token string, data map[string]any, outputFormat format.OutputFormat) error {
	body, err := c.preparePayload(manifest, serverURL, token, data)
	if err != nil {
		return err
	}

	var contentHandler func([]byte) error
	switch outputFormat {
	case format.Json:
		contentHandler = common.PrintJSON
	case format.Table:
		contentHandler = printDryRunResponseAsTable
	}

	return common.CallWorkerAPI(c.ctx, common.APICallParams{
		Method:      http.MethodPost,
		ServerURL:   serverURL,
		ServerToken: token,
		Body:        body,
		ProjectKey:  manifest.ProjectKey,
		Query: map[string]string{
			"debug": fmt.Sprint(manifest.Debug),
		},
		OkStatuses: []int{http.StatusOK},
		Path:       []string{"test", manifest.Name},
		OnContent:  contentHandler,
	})
}

func printDryRunResponseAsTable(responseBytes []byte) error {
	var data map[string]any
	if err := json.Unmarshal(responseBytes, &data); err != nil {
		return common.PrintJSON(responseBytes)
	}

	writer := common.NewCsvWriter()
	for k, v := range data {
		if err := writer.Write([]string{k, fmt.Sprint(v)}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func (c *dryRunHandler) preparePayload(manifest *model.Manifest, serverURL string, token string, data map[string]any) ([]byte, error) {
	payload := &dryRunRequest{Action: manifest.Action, Data: data}

	var err error

	payload.Code, err = common.ReadSourceCode(manifest)
	if err != nil {
		return nil, err
	}
	payload.Code = common.CleanImports(payload.Code)

	existingWorker, err := common.FetchWorkerDetails(c.ctx, serverURL, token, manifest.Name, manifest.ProjectKey)
	if err != nil {
		log.Warn(err.Error())
	}

	if !c.ctx.GetBoolFlagValue(model.FlagNoSecrets) {
		payload.StagedSecrets = common.PrepareSecretsUpdate(manifest, existingWorker)
	}

	return json.Marshal(&payload)
}

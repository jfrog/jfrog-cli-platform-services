package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"

	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

// Useful to capture output in tests
var (
	cliOut io.Writer = os.Stdout
	cliIn  io.Reader = os.Stdin
)

func prettifyJson(in []byte) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, in, "", "  "); err != nil {
		return in
	}
	return out.Bytes()
}

func outputApiResponse(res *http.Response, okStatus int) error {
	return processApiResponse(res, func(responseBytes []byte, statusCode int) error {
		var err error

		if res.StatusCode != okStatus {
			err = fmt.Errorf("command failed with status %d", res.StatusCode)
		}

		if err == nil {
			_, err = cliOut.Write(prettifyJson(responseBytes))
		} else if len(responseBytes) > 0 {
			// We will report the previous error, but we still want to display the response body
			if _, writeErr := cliOut.Write(prettifyJson(responseBytes)); writeErr != nil {
				log.Debug(fmt.Sprintf("Write error: %+v", writeErr))
			}
		}

		return err
	})
}

type stringFlagAware interface {
	GetStringFlagValue(string) string
}

// Extracts the project key and worker key from the command context. If the project key is not provided, it will be taken from the manifest.
// There workerKey could either be the first argument or the name in the manifest.
// The first argument will only be considered as the workerKey if total arguments are greater than minArgument.
func extractProjectAndKeyFromCommandContext(c stringFlagAware, args []string, minArguments int, onlyGeneric bool) (string, string, error) {
	var workerKey string

	projectKey := c.GetStringFlagValue(model.FlagProjectKey)

	if len(args) > 0 && len(args) > minArguments {
		workerKey = args[0]
	}

	if workerKey == "" || projectKey == "" {
		manifest, err := model.ReadManifest()
		if err != nil {
			return "", "", err
		}

		if err = manifest.Validate(); err != nil {
			return "", "", err
		}

		if onlyGeneric && manifest.Action != "GENERIC_EVENT" {
			return "", "", fmt.Errorf("only the GENERIC_EVENT actions are executable. Got %s", manifest.Action)
		}

		if workerKey == "" {
			workerKey = manifest.Name
		}

		if projectKey == "" {
			projectKey = manifest.ProjectKey
		}
	}

	return workerKey, projectKey, nil
}

func discardApiResponse(res *http.Response, okStatus int) error {
	return processApiResponse(res, func(content []byte, statusCode int) error {
		var err error
		if res.StatusCode != okStatus {
			err = fmt.Errorf("command failed with status %d", res.StatusCode)
		}
		return err
	})
}

func processApiResponse(res *http.Response, doWithContent func(content []byte, statusCode int) error) error {
	var err error
	var responseBytes []byte

	defer func() {
		if err = res.Body.Close(); err != nil {
			log.Debug(fmt.Sprintf("Error closing response body: %+v", err))
		}
	}()

	if res.ContentLength > 0 {
		responseBytes, err = io.ReadAll(res.Body)
		if err != nil {
			return err
		}
	} else {
		_, _ = io.Copy(io.Discard, res.Body)
	}

	if doWithContent == nil {
		return nil
	}

	return doWithContent(responseBytes, res.StatusCode)
}

func callWorkerApi(c *components.Context, serverUrl string, serverToken string, method string, body []byte, queryParams map[string]string, api ...string) (*http.Response, func(), error) {
	timeout, err := model.GetTimeoutParameter(c)
	if err != nil {
		return nil, nil, err
	}

	apiEndpoint := fmt.Sprintf("%sworker/api/v1/%s", utils.AddTrailingSlashIfNeeded(serverUrl), strings.Join(api, "/"))

	if queryParams != nil {
		var query string
		for key, value := range queryParams {
			if query != "" {
				query += "&"
			}
			query += fmt.Sprintf("%s=%s", key, url.QueryEscape(value))
		}
		if query != "" {
			apiEndpoint += "?" + query
		}
	}

	reqCtx, cancelReq := context.WithTimeout(context.Background(), timeout)

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(reqCtx, method, apiEndpoint, bodyReader)
	if err != nil {
		return nil, cancelReq, err
	}

	req.Header.Add("Authorization", "Bearer "+strings.TrimSpace(serverToken))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", coreutils.GetCliUserAgent())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, cancelReq, fmt.Errorf("request timed out after %s", timeout)
		}
		return nil, cancelReq, err
	}

	return res, cancelReq, nil
}

func callWorkerApiWithOutput(c *components.Context, serverUrl string, serverToken string, method string, body []byte, okStatus int, queryParams map[string]string, api ...string) error {
	res, discardReq, err := callWorkerApi(c, serverUrl, serverToken, method, body, queryParams, api...)
	if discardReq != nil {
		defer discardReq()
	}
	if err != nil {
		return err
	}
	return outputApiResponse(res, okStatus)
}

func callWorkerApiSilent(c *components.Context, serverUrl string, serverToken string, method string, body []byte, okStatus int, queryParams map[string]string, api ...string) error {
	res, discardReq, err := callWorkerApi(c, serverUrl, serverToken, method, body, queryParams, api...)
	if discardReq != nil {
		defer discardReq()
	}
	if err != nil {
		return err
	}
	return discardApiResponse(res, okStatus)
}

// fetchWorkerDetails Fetch a worker by its name. Returns nil if the worker does not exist (statusCode=404). Any other statusCode other than 200 will result as an error.
func fetchWorkerDetails(c *components.Context, serverUrl string, accessToken string, workerKey string, projectKey string) (*model.WorkerDetails, error) {
	queryParams := make(map[string]string)
	if projectKey != "" {
		queryParams["projectKey"] = projectKey
	}

	res, discardReq, err := callWorkerApi(c, serverUrl, accessToken, http.MethodGet, nil, queryParams, "workers", workerKey)
	if discardReq != nil {
		defer discardReq()
	}
	if err != nil {
		return nil, err
	}

	var details *model.WorkerDetails

	err = processApiResponse(res, func(content []byte, statusCode int) error {
		if statusCode == http.StatusOK {
			unmarshalled := new(model.WorkerDetails)
			err := json.Unmarshal(content, unmarshalled)
			if err == nil {
				details = unmarshalled
				return nil
			}
			return err
		}
		if statusCode != http.StatusNotFound {
			return fmt.Errorf("fetch worker '%s' failed with status %d", workerKey, statusCode)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return details, nil
}

func prepareSecretsUpdate(mf *model.Manifest, existingWorker *model.WorkerDetails) []*model.Secret {
	// We will detect removed secrets
	removedSecrets := map[string]any{}
	if existingWorker != nil {
		for _, existingSecret := range existingWorker.Secrets {
			removedSecrets[existingSecret.Key] = struct{}{}
		}
	}

	var secrets []*model.Secret

	// Secrets should have already been decoded
	for secretName, secretValue := range mf.Secrets {
		_, secretExists := removedSecrets[secretName]
		if secretExists {
			// To take into account the local value of a secret
			secrets = append(secrets, &model.Secret{Key: secretName, MarkedForRemoval: true})
		}
		delete(removedSecrets, secretName)
		secrets = append(secrets, &model.Secret{Key: secretName, Value: secretValue})
	}

	for removedSecret := range removedSecrets {
		secrets = append(secrets, &model.Secret{Key: removedSecret, MarkedForRemoval: true})
	}

	return secrets
}

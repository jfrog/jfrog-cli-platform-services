package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"

	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

// Useful to capture output in tests
var (
	cliOut        io.Writer = os.Stdout
	cliIn         io.Reader = os.Stdin
	importPattern           = regexp.MustCompile(`(?ms)^\s*(import\s+[^;]+;\s*)(.*)$`)
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

func callWorkerApi(c *components.Context, serverUrl string, serverToken string, method string, body []byte, api ...string) (*http.Response, func(), error) {
	timeout, err := model.GetTimeoutParameter(c)
	if err != nil {
		return nil, nil, err
	}

	url := fmt.Sprintf("%sworker/api/v1/%s", utils.AddTrailingSlashIfNeeded(serverUrl), strings.Join(api, "/"))

	reqCtx, cancelReq := context.WithTimeout(context.Background(), timeout)

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(reqCtx, method, url, bodyReader)
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

func callWorkerApiWithOutput(c *components.Context, serverUrl string, serverToken string, method string, body []byte, okStatus int, api ...string) error {
	res, discardReq, err := callWorkerApi(c, serverUrl, serverToken, method, body, api...)
	if discardReq != nil {
		defer discardReq()
	}
	if err != nil {
		return err
	}
	return outputApiResponse(res, okStatus)
}

func callWorkerApiSilent(c *components.Context, serverUrl string, serverToken string, method string, body []byte, okStatus int, api ...string) error {
	res, discardReq, err := callWorkerApi(c, serverUrl, serverToken, method, body, api...)
	if discardReq != nil {
		defer discardReq()
	}
	if err != nil {
		return err
	}
	return discardApiResponse(res, okStatus)
}

// fetchWorkerDetails Fetch a worker by its name. Returns nil if the worker does not exist (statusCode=404). Any other statusCode other than 200 will result as an error.
func fetchWorkerDetails(c *components.Context, serverUrl string, accessToken string, workerKey string) (*model.WorkerDetails, error) {
	res, discardReq, err := callWorkerApi(c, serverUrl, accessToken, http.MethodGet, nil, "workers", workerKey)
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

func cleanImports(source string) string {
	out := source
	match := importPattern.FindAllStringSubmatch(out, -1)
	for len(match) == 1 && len(match[0]) == 3 {
		out = match[0][2]
		match = importPattern.FindAllStringSubmatch(out, -1)
	}
	return out
}

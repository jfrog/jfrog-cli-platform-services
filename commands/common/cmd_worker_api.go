package common

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// FetchWorkerDetails Fetch a worker by its name. Returns nil if the worker does not exist (statusCode=404). Any other statusCode other than 200 will result as an error.
func FetchWorkerDetails(c model.IntFlagProvider, serverUrl string, accessToken string, workerKey string, projectKey string) (*model.WorkerDetails, error) {
	details := new(model.WorkerDetails)

	err := CallWorkerApi(c, ApiCallParams{
		Method:      http.MethodGet,
		ServerUrl:   serverUrl,
		ServerToken: accessToken,
		OkStatuses:  []int{http.StatusOK},
		ProjectKey:  projectKey,
		Path:        []string{"workers", workerKey},
		OnContent: func(content []byte) error {
			return json.Unmarshal(content, details)
		},
	})
	if err != nil {
		var apiErr *ApiError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	return details, nil
}

func FetchActions(c model.IntFlagProvider, serverUrl string, accessToken string, projectKey string) (ActionsMetadata, error) {
	metadata := make(ActionsMetadata, 0)

	err := CallWorkerApi(c, ApiCallParams{
		Method:      http.MethodGet,
		ServerUrl:   serverUrl,
		ServerToken: accessToken,
		OkStatuses:  []int{http.StatusOK},
		ProjectKey:  projectKey,
		ApiVersion:  ApiVersionV2,
		Path:        []string{"actions"},
		OnContent: func(content []byte) error {
			if len(content) == 0 {
				log.Debug("No actions returned from the server")
				return nil
			}
			return json.Unmarshal(content, &metadata)
		},
	})
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

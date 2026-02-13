package common

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// FetchWorkerDetails Fetch a worker by its name. Returns nil if the worker does not exist (statusCode=404). Any other statusCode other than 200 will result as an error.
func FetchWorkerDetails(c model.IntFlagProvider, serverURL string, accessToken string, workerKey string, projectKey string) (*model.WorkerDetails, error) {
	details := new(model.WorkerDetails)
	err := CallWorkerAPI(c, APICallParams{
		Method:      http.MethodGet,
		ServerURL:   serverURL,
		ServerToken: accessToken,
		OkStatuses:  []int{http.StatusOK, http.StatusNotFound},
		ProjectKey:  projectKey,
		Path:        []string{"workers", workerKey},
		OnContent: func(content []byte) error {
			if len(content) == 0 {
				return nil
			}
			log.Info(fmt.Sprintf("Worker %s details returned from the server", details.Key))
			return json.Unmarshal(content, details)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot fetch worker details: %w", err)
	}

	if details.Key == "" {
		log.Info(fmt.Sprintf("Worker %s does not exist", workerKey))
		return nil, nil
	}
	return details, nil
}

func FetchActions(c model.IntFlagProvider, serverURL string, accessToken string, projectKey string) (ActionsMetadata, error) {
	metadata := make(ActionsMetadata, 0)

	err := CallWorkerAPI(c, APICallParams{
		Method:      http.MethodGet,
		ServerURL:   serverURL,
		ServerToken: accessToken,
		OkStatuses:  []int{http.StatusOK},
		ProjectKey:  projectKey,
		APIVersion:  APIVersionV2,
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

func FetchOptions(c model.IntFlagProvider, serverURL string, accessToken string) (*OptionsMetadata, error) {
	metadata := new(OptionsMetadata)

	err := CallWorkerAPI(c, APICallParams{
		Method:      http.MethodGet,
		ServerURL:   serverURL,
		ServerToken: accessToken,
		OkStatuses:  []int{http.StatusOK},
		APIVersion:  APIVersionV1,
		Path:        []string{"options"},
		OnContent: func(content []byte) error {
			if len(content) == 0 {
				log.Debug("No options returned from the server")
				return nil
			}
			return json.Unmarshal(content, &metadata)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot fetch options: %w", err)
	}
	return metadata, nil
}

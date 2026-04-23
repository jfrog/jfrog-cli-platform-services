// Package common provides shared utilities for JFrog platform services commands.
package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/jfrog/jfrog-client-go/utils"
)

type apiVersion int

const (
	APIVersionV1 apiVersion = iota + 1
	APIVersionV2 apiVersion = 2
)

type APIContentHandler func(content []byte) error

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

func apiError(status int, message string, args ...any) *APIError {
	return &APIError{
		StatusCode: status,
		Message:    fmt.Sprintf(message, args...),
	}
}

type APICallParams struct {
	Method      string
	ServerURL   string
	ServerToken string
	Body        []byte
	Query       map[string]string
	Path        []string
	ProjectKey  string
	APIVersion  apiVersion
	OkStatuses  []int
	OnContent   APIContentHandler
}

func CallWorkerAPI(c model.IntFlagProvider, params APICallParams) error {
	timeout, err := model.GetTimeoutParameter(c)
	if err != nil {
		return apiError(http.StatusInternalServerError, "%+v", err)
	}

	apiVersion := APIVersionV1
	if params.APIVersion != 0 {
		apiVersion = params.APIVersion
	}

	apiEndpoint := fmt.Sprintf("%sworker/api/v%d/%s", utils.AddTrailingSlashIfNeeded(params.ServerURL), apiVersion, strings.Join(params.Path, "/"))

	q := url.Values{}

	if params.ProjectKey != "" {
		q.Set("projectKey", params.ProjectKey)
	}

	for key, value := range params.Query {
		q.Set(key, value)
	}

	reqCtx, cancelReq := context.WithTimeout(context.Background(), timeout)
	defer cancelReq()

	var bodyReader io.Reader
	if params.Body != nil {
		bodyReader = bytes.NewBuffer(params.Body)
	}

	if len(q) > 0 {
		apiEndpoint += "?" + q.Encode()
	}

	req, err := http.NewRequestWithContext(reqCtx, params.Method, apiEndpoint, bodyReader)
	if err != nil {
		return apiError(http.StatusInternalServerError, "failed to create request: %+v", err)
	}

	req.Header.Add("Authorization", "Bearer "+strings.TrimSpace(params.ServerToken))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", coreutils.GetCliUserAgent())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return apiError(http.StatusRequestTimeout, "request timed out after %s", timeout)
		}
		return apiError(http.StatusInternalServerError, "unexpected error: %+v", err)
	}

	if slices.Index(params.OkStatuses, res.StatusCode) == -1 {
		// We the response contains json content, we will print it
		_ = processAPIResponse(res, printJSONOrLogError)
		return apiError(res.StatusCode, "command %s %s returned an unexpected status code %d", params.Method, apiEndpoint, res.StatusCode)
	}

	return processAPIResponse(res, params.OnContent)
}

func processAPIResponse(res *http.Response, doWithContent func(content []byte) error) error {
	var err error
	var responseBytes []byte

	defer CloseQuietly(res.Body)

	if res.ContentLength == 0 {
		_, _ = io.Copy(io.Discard, res.Body)
	} else {
		responseBytes, err = io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("cannot read response content: %+v", err)
		}
	}

	if doWithContent == nil {
		return nil
	}

	return doWithContent(responseBytes)
}

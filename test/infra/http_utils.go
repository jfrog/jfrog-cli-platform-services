//go:build itest

package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	contentTypeText = "text/plain"
)

type basicAuth struct {
	username string
	password string
}

type HttpRequest struct {
	it         *Test
	tokenAuth  string
	basicAuth  *basicAuth
	bodyReader io.Reader
	headers    map[string]string
	reqContext context.Context
	url        string
}

type HttpResponse struct {
	it         *Test
	response   *http.Response
	bodyRead   bool
	cachedBody string
}

type HttpExecutor struct {
	it              *Test
	request         *http.Request
	retryOnStatuses []int
	retryBackoff    time.Duration
	retryTimeout    time.Duration
}

func (h *HttpRequest) getUrl(endpoint string) string {
	baseUrl := h.url
	if h.url == "" {
		return "http://localhost:8082"
	}
	return strings.TrimSuffix(baseUrl, "/") + "/" + strings.TrimPrefix(endpoint, "/")
}

func (h *HttpRequest) Get(endpoint string) *HttpExecutor {
	req, err := http.NewRequestWithContext(h.getRequestContext(), http.MethodGet, h.getUrl(endpoint), h.bodyReader)
	require.NoError(h.it, err)
	h.setUpRequest(req)
	return h.executor(req)
}

func (h *HttpRequest) setUpRequest(req *http.Request) {
	if h.basicAuth != nil {
		req.SetBasicAuth(h.basicAuth.username, h.basicAuth.password)
	}
	for k, v := range h.headers {
		req.Header.Add(k, v)
	}
	if h.tokenAuth != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", h.tokenAuth))
	}
}

func (h *HttpRequest) WithURL(url string) *HttpRequest {
	h.url = url
	return h
}

func (h *HttpRequest) Post(endpoint string) *HttpExecutor {
	req, err := http.NewRequestWithContext(h.getRequestContext(), http.MethodPost, h.getUrl(endpoint), h.bodyReader)
	require.NoError(h.it, err)
	h.setUpRequest(req)
	return h.executor(req)
}

func (h *HttpRequest) Put(endpoint string) *HttpExecutor {
	req, err := http.NewRequestWithContext(h.getRequestContext(), http.MethodPut, h.getUrl(endpoint), h.bodyReader)
	require.NoError(h.it, err)
	h.setUpRequest(req)
	return h.executor(req)
}

func (h *HttpRequest) Putf(endpoint string, args ...any) *HttpExecutor {
	return h.Put(fmt.Sprintf(endpoint, args...))
}

func (h *HttpRequest) Delete(endpoint string) *HttpExecutor {
	req, err := http.NewRequestWithContext(h.getRequestContext(), http.MethodDelete, h.getUrl(endpoint), h.bodyReader)
	require.NoError(h.it, err)
	h.setUpRequest(req)
	return h.executor(req)
}

func (h *HttpRequest) executor(req *http.Request) *HttpExecutor {
	return &HttpExecutor{
		it:      h.it,
		request: req,
	}
}

func (h *HttpExecutor) WithRetries(backoff, timeout time.Duration, statuses ...int) *HttpExecutor {
	h.retryBackoff = backoff
	h.retryTimeout = timeout
	h.retryOnStatuses = statuses
	return h
}

func (h *HttpExecutor) Do() *HttpResponse {
	resp, err := h.doWithRetries()
	require.NoError(h.it, err)
	return resp
}

func (h *HttpExecutor) DoAndCaptureError() (*HttpResponse, error) {
	return h.doWithRetries()
}

func (h *HttpExecutor) doWithRetries() (*HttpResponse, error) {
	start := time.Now()

	resp, err := http.DefaultClient.Do(h.request)
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(start)
	backoff := max(250*time.Millisecond, h.retryBackoff)

	for slices.Index(h.retryOnStatuses, resp.StatusCode) > -1 && elapsed < h.retryTimeout {
		time.Sleep(backoff)
		resp, err = http.DefaultClient.Do(h.request)
		if err != nil {
			return nil, err
		}
		elapsed = time.Since(start)
	}

	return &HttpResponse{
		it:       h.it,
		response: resp,
	}, nil
}

func (h *HttpRequest) ReadResponse(resp *http.Response) (string, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			h.it.Logf("Error when closing HTTP response body: %+v", err)
		}
	}()
	body := string(bodyBytes)
	return body, nil
}

func (h *HttpRequest) WithBasicAuth(username string, password string) *HttpRequest {
	h.basicAuth = &basicAuth{
		username: username,
		password: password,
	}
	return h
}

func (h *HttpRequest) WithFormData(params map[string]string) *HttpRequest {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	h.headers["Content-Type"] = "application/x-www-form-urlencoded"
	return h.WithBody(strings.NewReader(values.Encode()))
}

func (h *HttpRequest) WithJsonData(data any) *HttpRequest {
	jsonBytes, err := json.Marshal(data)
	require.NoError(h.it, err)
	return h.WithJsonBytes(jsonBytes)
}

func (h *HttpRequest) WithJsonBytes(jsonBytes []byte) *HttpRequest {
	return h.WithJsonString(string(jsonBytes))
}

func (h *HttpRequest) WithJsonString(jsonString string) *HttpRequest {
	h.headers["Content-Type"] = "application/json; charset=UTF-8"
	return h.WithBody(strings.NewReader(jsonString))
}

func (h *HttpRequest) WithBody(r io.Reader) *HttpRequest {
	h.bodyReader = r
	return h
}

func (h *HttpRequest) WithHeader(key, value string) *HttpRequest {
	h.headers[key] = value
	return h
}

func (h *HttpRequest) WithHeaders(headers map[string]string) *HttpRequest {
	for k, v := range headers {
		h.WithHeader(k, v)
	}
	return h
}

func (h *HttpRequest) WithTokenAuth(token string) *HttpRequest {
	h.tokenAuth = token
	return h
}

func (h *HttpRequest) WithAccessToken() *HttpRequest {
	return h.WithTokenAuth(h.it.AccessToken)
}

func (h *HttpRequest) getRequestContext() context.Context {
	if h.reqContext == nil {
		return context.Background()
	}
	return h.reqContext
}

func (r *HttpResponse) AsString() string {
	if !r.bodyRead {
		bodyBytes, err := io.ReadAll(r.response.Body)
		require.NoError(r.it, err)
		defer func() {
			if err = r.response.Body.Close(); err != nil {
				r.it.Logf("Error when closing HTTP response body: %+v", err)
			}
		}()
		body := string(bodyBytes)
		r.bodyRead = true
		r.cachedBody = body
	}
	return r.cachedBody
}

func (r *HttpResponse) asJsonString() string {
	r.IsJsonContent()
	return r.AsString()
}

func (r *HttpResponse) AsJson() map[string]interface{} {
	r.IsJsonContent()

	result := make(map[string]interface{})

	responseBody := r.AsString()
	err := json.Unmarshal([]byte(responseBody), &result)
	require.NoError(r.it, err, "Cannot parse to JSON: %v", responseBody)
	return result
}

func (r *HttpResponse) AsObject(out interface{}) {
	r.IsJsonContent()

	responseBody := r.AsString()
	err := json.Unmarshal([]byte(responseBody), &out)
	require.NoError(r.it, err, "Cannot parse to JSON: %v", responseBody)
}

func (r *HttpResponse) AsBytes() []byte {
	if !r.bodyRead {
		bodyBytes, err := io.ReadAll(r.response.Body)
		require.NoError(r.it, err)
		defer func() {
			if err = r.response.Body.Close(); err != nil {
				r.it.Logf("Error when closing HTTP response body: %+v", err)
			}
		}()
		r.bodyRead = true
		return bodyBytes
	}
	return []byte(r.cachedBody)
}

func (r *HttpResponse) Status(expectedStatus int) *HttpResponse {
	require.Equal(r.it, expectedStatus, r.response.StatusCode, "Response body is: %v", r.AsString())
	return r
}

func (r *HttpResponse) StatusCode() int {
	return r.response.StatusCode
}

func (r *HttpResponse) IsOk() *HttpResponse {
	r.Status(http.StatusOK)
	return r
}

func (r *HttpResponse) IsUnauthorized() *HttpResponse {
	r.Status(http.StatusUnauthorized)
	return r
}

func (r *HttpResponse) IsForbidden() *HttpResponse {
	r.Status(http.StatusForbidden)
	return r
}

func (r *HttpResponse) IsBadRequest() *HttpResponse {
	r.Status(http.StatusBadRequest)
	return r
}

func (r *HttpResponse) IsInternalServerError() *HttpResponse {
	r.Status(http.StatusInternalServerError)
	return r
}

func (r *HttpResponse) IsServiceUnavailable() *HttpResponse {
	r.Status(http.StatusServiceUnavailable)
	return r
}

func (r *HttpResponse) IsCreated() *HttpResponse {
	r.Status(http.StatusCreated)
	return r
}

func (r *HttpResponse) IsNoContent() *HttpResponse {
	r.Status(http.StatusNoContent)
	require.Empty(r.it, r.AsString())
	return r
}

func (r *HttpResponse) IsConflict() *HttpResponse {
	r.Status(http.StatusConflict)
	return r
}

func (r *HttpResponse) IsNotFound() *HttpResponse {
	r.Status(http.StatusNotFound)
	return r
}

func (r *HttpResponse) IsJsonContent() *HttpResponse {
	return r.IsContent("application/json")
}

func (r *HttpResponse) IsTextContent() *HttpResponse {
	return r.IsContent(contentTypeText)
}

func (r *HttpResponse) IsContent(expectedContentType string) *HttpResponse {
	contentType := r.response.Header.Get("Content-Type")
	require.Condition(r.it,
		func() bool {
			return strings.HasPrefix(contentType, expectedContentType)
		},
		"Content-Type should be %v, not: %v", expectedContentType,
		contentType)
	return r
}

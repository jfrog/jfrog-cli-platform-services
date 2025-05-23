//go:build test

package common

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jfrog/go-mockhttp"
	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	EndpointExecutionHistory = "/worker/api/v1/execution_history"
)

type BodyValidator func(t require.TestingT, content []byte)

type workerDeployPayload struct {
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

func ValidateJson(expected any) BodyValidator {
	return ValidateJsonFunc(expected, func(in any) any {
		return in
	})
}

func ValidateJsonFunc(expected any, extractPayload func(in any) any) BodyValidator {
	return func(t require.TestingT, content []byte) {
		var got interface{}
		err := json.Unmarshal(content, &got)
		require.NoError(t, err)
		payload := extractPayload(got)
		assert.Equal(t, expected, payload)
	}
}

func NewMockWorkerServer(t *testing.T, stubs ...*ServerStub) (*mockhttp.Server, string) {
	token := uuid.NewString()

	var allEndpoints []mockhttp.ServerEndpoint

	for _, stub := range stubs {
		if stub.token == "" {
			stub.token = token
		}
		for _, endpoint := range stub.endpoints {
			allEndpoints = append(allEndpoints, endpoint)
		}
	}

	server := mockhttp.StartServer(mockhttp.WithEndpoints(allEndpoints...), mockhttp.WithName("worker"))

	TestSetEnv(t, model.EnvKeyServerUrl, server.BaseUrl())
	TestSetEnv(t, model.EnvKeyAccessToken, token)
	TestSetEnv(t, model.EnvKeySecretsPassword, SecretPassword)

	t.Cleanup(server.Close)

	return server, token
}

type queryParamStub struct {
	value string
	paths []string
}

type ExecutionHistoryResultEntryStub struct {
	Result string `json:"result"`
	Logs   string `json:"logs"`
}

type ExecutionHistoryEntryStub struct {
	Start   time.Time                       `json:"start"`
	End     time.Time                       `json:"end"`
	TestRun bool                            `json:"testRun"`
	Result  ExecutionHistoryResultEntryStub `json:"entries"`
}

type ExecutionHistoryStub []*ExecutionHistoryEntryStub

func NewServerStub(t *testing.T) *ServerStub {
	return &ServerStub{
		test:             t,
		workers:          map[string]*model.WorkerDetails{},
		queryParams:      map[string]queryParamStub{},
		executionHistory: map[string]ExecutionHistoryStub{},
	}
}

type ServerStub struct {
	test             *testing.T
	waitFor          time.Duration
	token            string
	projectKey       *queryParamStub
	workers          map[string]*model.WorkerDetails
	executionHistory map[string]ExecutionHistoryStub
	endpoints        []mockhttp.ServerEndpoint
	queryParams      map[string]queryParamStub
}

func (s *ServerStub) WithT(t *testing.T) *ServerStub {
	s.test = t
	return s
}

func (s *ServerStub) WithDelay(waitFor time.Duration) *ServerStub {
	s.waitFor = waitFor
	return s
}

func (s *ServerStub) WithToken(token string) *ServerStub {
	s.token = token
	return s
}

func (s *ServerStub) WithProjectKey(projectKey string, paths ...string) *ServerStub {
	s.projectKey = &queryParamStub{
		value: projectKey,
		paths: paths,
	}
	return s
}

func (s *ServerStub) WithQueryParam(name, value string, paths ...string) *ServerStub {
	s.queryParams[name] = queryParamStub{value: value, paths: paths}
	return s
}

func (s *ServerStub) WithWorkers(workers ...*model.WorkerDetails) *ServerStub {
	for _, worker := range workers {
		s.workers[worker.Key] = worker
	}
	return s
}

func (s *ServerStub) WithWorkerExecutionHistory(workerKey string, history ExecutionHistoryStub) *ServerStub {
	s.executionHistory[workerKey] = history
	return s
}

func (s *ServerStub) WithCreateEndpoint(validateBody BodyValidator) *ServerStub {
	s.endpoints = append(s.endpoints,
		mockhttp.NewServerEndpoint().
			When(
				mockhttp.Request().POST("/worker/api/v2/workers"),
			).
			HandleWith(s.handleSave(http.StatusCreated, validateBody)),
	)
	return s
}

func (s *ServerStub) WithDefaultActionsMetadataEndpoint() *ServerStub {
	return s.WithActionsMetadataEndpoint(LoadSampleActions(s.test))
}

func (s *ServerStub) WithActionsMetadataEndpoint(metadata ActionsMetadata) *ServerStub {
	s.endpoints = append(s.endpoints,
		mockhttp.NewServerEndpoint().
			When(
				mockhttp.Request().GET("/worker/api/v2/actions"),
			).
			HandleWith(s.handleGetAllMetadata(metadata)),
	)
	return s
}

func (s *ServerStub) WithUpdateEndpoint(validateBody BodyValidator) *ServerStub {
	s.endpoints = append(s.endpoints,
		mockhttp.NewServerEndpoint().
			When(
				mockhttp.Request().PUT("/worker/api/v2/workers"),
			).
			HandleWith(s.handleSave(http.StatusNoContent, validateBody)),
	)
	return s
}

func (s *ServerStub) WithDeleteEndpoint() *ServerStub {
	s.endpoints = append(s.endpoints,
		mockhttp.NewServerEndpoint().
			When(
				mockhttp.Request().
					Method(http.MethodDelete).
					PathMatches(regexp.MustCompile(`/worker/api/v1/workers/[^\\]+`)),
			).
			HandleWith(s.handleDelete),
	)
	return s
}

func (s *ServerStub) WithTestEndpoint(validateBody BodyValidator, responseBody any, status ...int) *ServerStub {
	okStatus := http.StatusOK
	if len(status) > 0 {
		okStatus = status[0]
	}
	s.endpoints = append(s.endpoints,
		mockhttp.NewServerEndpoint().
			When(
				mockhttp.Request().
					Method(http.MethodPost).
					PathMatches(regexp.MustCompile(`/worker/api/v1/test/[^\\]+`)),
			).
			HandleWith(s.handle(okStatus, validateBody, responseBody)),
	)
	return s
}

func (s *ServerStub) WithExecuteEndpoint(validateBody BodyValidator, responseBody any, status ...int) *ServerStub {
	okStatus := http.StatusOK
	if len(status) > 0 {
		okStatus = status[0]
	}
	s.endpoints = append(s.endpoints,
		mockhttp.NewServerEndpoint().
			When(
				mockhttp.Request().
					Method(http.MethodPost).
					PathMatches(regexp.MustCompile(`/worker/api/v1/execute/[^\\]+`)),
			).
			HandleWith(s.handle(okStatus, validateBody, responseBody)),
	)
	return s
}

func (s *ServerStub) WithGetOneEndpoint() *ServerStub {
	s.endpoints = append(s.endpoints,
		mockhttp.NewServerEndpoint().
			When(
				mockhttp.Request().
					Method(http.MethodGet).
					PathMatches(regexp.MustCompile(`/worker/api/v1/workers/[^\\]+`)),
			).
			HandleWith(s.handleGetOne),
	)
	return s
}

func (s *ServerStub) WithGetAllEndpoint() *ServerStub {
	s.endpoints = append(s.endpoints,
		mockhttp.NewServerEndpoint().
			When(
				mockhttp.Request().
					Method(http.MethodGet).
					PathMatches(regexp.MustCompile(`/worker/api/v1/workers(\?.+)?`)),
			).
			HandleWith(s.handleGetAll),
	)
	return s
}

func (s *ServerStub) WithGetExecutionHistoryEndpoint() *ServerStub {
	s.endpoints = append(s.endpoints,
		mockhttp.NewServerEndpoint().
			When(
				mockhttp.Request().
					Method(http.MethodGet).
					Path(EndpointExecutionHistory),
			).
			HandleWith(s.handleGetExecutionHistory),
	)
	return s
}

func (s *ServerStub) WithOptionsEndpoint() *ServerStub {
	s.endpoints = append(s.endpoints,
		mockhttp.NewServerEndpoint().
			When(
				mockhttp.Request().GET("/worker/api/v1/options"),
			).
			HandleWith(s.handleGetOptions),
	)
	return s
}

func (s *ServerStub) handleGetAll(res http.ResponseWriter, req *http.Request) {
	s.applyDelay()

	if !s.validateToken(res, req) {
		return
	}

	if !s.validateProjectKey(res, req) {
		return
	}

	if !s.validateQueryParams(res, req) {
		return
	}

	res.WriteHeader(http.StatusOK)

	action := req.URL.Query().Get("action")

	workers := make([]*model.WorkerDetails, 0, len(s.workers))
	for _, worker := range s.workers {
		if action == "" || worker.Action == action {
			workers = append(workers, worker)
		}
	}

	_, err := res.Write([]byte(MustJsonMarshal(s.test, map[string]any{"workers": workers})))
	require.NoError(s.test, err)
}

func (s *ServerStub) handleGetOne(res http.ResponseWriter, req *http.Request) {
	s.applyDelay()

	if !s.validateToken(res, req) {
		return
	}

	if !s.validateProjectKey(res, req) {
		return
	}

	if !s.validateQueryParams(res, req) {
		return
	}

	var workerKey string

	path := strings.Split(req.URL.Path, "/")
	if len(path) > 1 {
		workerKey = path[len(path)-1]
	}

	workerDetails, workerExists := s.workers[workerKey]
	if !workerExists {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	_, err := res.Write([]byte(MustJsonMarshal(s.test, workerDetails)))
	require.NoError(s.test, err)
}

func (s *ServerStub) handleGetExecutionHistory(res http.ResponseWriter, req *http.Request) {
	s.applyDelay()

	if !s.validateToken(res, req) {
		return
	}

	if !s.validateProjectKey(res, req) {
		return
	}

	if !s.validateQueryParams(res, req) {
		return
	}

	workerKey := req.URL.Query().Get("workerKey")

	executionHistory, hasHistory := s.executionHistory[workerKey]
	if !hasHistory {
		executionHistory = ExecutionHistoryStub{}
	}

	showTestRun := req.URL.Query().Get("showTestRun") == "true"

	newHistory := make(ExecutionHistoryStub, 0, len(executionHistory))
	for _, entry := range executionHistory {
		if entry.TestRun {
			if showTestRun {
				newHistory = append(newHistory, entry)
			}
		} else {
			newHistory = append(newHistory, entry)
		}
	}
	executionHistory = newHistory

	res.Header().Set("Content-Type", "application/json")

	_, err := res.Write([]byte(MustJsonMarshal(s.test, executionHistory)))
	require.NoError(s.test, err)
}

func (s *ServerStub) handleDelete(res http.ResponseWriter, req *http.Request) {
	s.applyDelay()

	if !s.validateToken(res, req) {
		return
	}

	if !s.validateQueryParams(res, req) {
		return
	}

	var workerKey string

	path := strings.Split(req.URL.Path, "/")
	if len(path) > 1 {
		workerKey = path[len(path)-1]
	}

	_, workerExists := s.workers[workerKey]
	if !workerExists {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	res.WriteHeader(http.StatusNoContent)
}

func (s *ServerStub) handleSave(status int, validateBody BodyValidator) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		s.applyDelay()

		if !s.validateToken(res, req) {
			return
		}

		if !s.validateQueryParams(res, req) {
			return
		}

		content, err := io.ReadAll(req.Body)
		require.NoError(s.test, err)

		if validateBody != nil {
			validateBody(s.test, content)
		}

		worker := workerDeployPayload{}

		err = json.Unmarshal(content, &worker)
		require.NoError(s.test, err)

		workerDetails := mapWorkerSentToWorkerDetails(worker)
		s.workers[workerDetails.Key] = workerDetails

		res.WriteHeader(status)
	}
}

func (s *ServerStub) handleGetAllMetadata(metadata ActionsMetadata) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		s.applyDelay()

		if !s.validateToken(res, req) {
			return
		}

		if !s.validateProjectKey(res, req) {
			return
		}

		if !s.validateQueryParams(res, req) {
			return
		}

		res.WriteHeader(http.StatusOK)

		res.Header().Set("Content-Type", "application/json")

		_, err := res.Write([]byte(MustJsonMarshal(s.test, metadata)))
		if err != nil {
			s.test.Logf("Failed to write response: %v", err)
		}
	}
}

func (s *ServerStub) handleGetOptions(res http.ResponseWriter, req *http.Request) {
	s.applyDelay()

	if !s.validateToken(res, req) {
		return
	}

	res.WriteHeader(http.StatusOK)

	options := LoadSampleOptions(s.test)
	_, err := res.Write([]byte(MustJsonMarshal(s.test, options)))
	require.NoError(s.test, err)
}

func (s *ServerStub) handle(status int, validateBody BodyValidator, responseBody any) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		s.applyDelay()

		if !s.validateToken(res, req) {
			return
		}

		if !s.validateQueryParams(res, req) {
			return
		}

		if validateBody != nil {
			content, err := io.ReadAll(req.Body)
			require.NoError(s.test, err)
			validateBody(s.test, content)
		}

		res.WriteHeader(status)

		if responseBody != nil {
			res.Header().Set("Content-Type", "application/json")
			response, err := json.Marshal(responseBody)
			require.NoError(s.test, err)
			_, err = res.Write(response)
			require.NoError(s.test, err)
		}
	}
}

func (s *ServerStub) validateToken(res http.ResponseWriter, req *http.Request) bool {
	if s.token != "" {
		if req.Header.Get("Authorization") != "Bearer "+s.token {
			res.WriteHeader(http.StatusForbidden)
			return false
		}
	}
	return true
}

func (s *ServerStub) validateProjectKey(res http.ResponseWriter, req *http.Request) bool {
	if s.projectKey != nil && (len(s.projectKey.paths) == 0 || slices.Contains(s.projectKey.paths, req.URL.Path)) {
		gotProjectKey := req.URL.Query().Get("projectKey")
		if s.projectKey.value == gotProjectKey {
			return true
		}
		res.WriteHeader(http.StatusForbidden)
		assert.FailNow(s.test, "Invalid projectKey")
		return false
	}
	return true
}

func (s *ServerStub) validateQueryParams(res http.ResponseWriter, req *http.Request) bool {
	for key, stub := range s.queryParams {
		if len(stub.paths) > 0 && !slices.Contains(stub.paths, req.URL.Path) {
			// We only check the query param if the path is in the list
			continue
		}

		gotValue := req.URL.Query().Get(key)
		if stub.value == gotValue {
			continue
		}

		res.WriteHeader(http.StatusBadRequest)

		assert.FailNow(s.test, fmt.Sprintf("Invalid query params %s want=%s, got=%s", key, stub, gotValue))

		return false
	}
	return true
}

func (s *ServerStub) validateHeader(res http.ResponseWriter, req *http.Request, name, value string) bool {
	if req.Header.Get(name) != value {
		res.WriteHeader(http.StatusBadRequest)
		return false
	}
	return true
}

func (s *ServerStub) applyDelay() {
	if s.waitFor > 0 {
		time.Sleep(s.waitFor)
	}
}

func mapWorkerSentToWorkerDetails(workerSent workerDeployPayload) *model.WorkerDetails {
	return &model.WorkerDetails{
		Key:            workerSent.Key,
		Description:    workerSent.Description,
		Enabled:        workerSent.Enabled,
		Debug:          workerSent.Debug,
		SourceCode:     workerSent.SourceCode,
		Action:         workerSent.Action.Name, // Map Action.Name
		FilterCriteria: workerSent.FilterCriteria,
		Secrets:        workerSent.Secrets,
		ProjectKey:     workerSent.ProjectKey,
	}
}

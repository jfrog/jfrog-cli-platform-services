//go:build test
// +build test

package common

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallWorkerAPI(t *testing.T) {
	tests := []struct {
		name    string
		stub    *ServerStub
		params  APICallParams
		ctx     model.IntFlagProvider
		wantErr string
	}{
		{
			name: "success",
			stub: NewServerStub(t).WithGetAllEndpoint(),
			params: APICallParams{
				Method:     "GET",
				Path:       []string{"workers"},
				OkStatuses: []int{http.StatusOK},
			},
		},
		{
			name: "unexpected status",
			stub: NewServerStub(t).WithGetAllEndpoint(),
			params: APICallParams{
				Method:     "GET",
				Path:       []string{"workers"},
				OkStatuses: []int{http.StatusNoContent},
			},
			wantErr: `command GET .+/worker/api/v1/workers returned an unexpected status code 200`,
		},
		{
			name: "cancel on timeout",
			stub: NewServerStub(t).WithDelay(time.Second).WithGetAllEndpoint(),
			params: APICallParams{
				Method:     "GET",
				Path:       []string{"workers"},
				OkStatuses: []int{http.StatusNoContent},
			},
			ctx:     IntFlagMap{model.FlagTimeout: 250},
			wantErr: `request timed out after 250ms`,
		},
		{
			name: "add query params",
			stub: NewServerStub(t).
				WithGetAllEndpoint().
				WithQueryParam("a", "1").
				WithQueryParam("b", "2"),
			params: APICallParams{
				Method:     "GET",
				Path:       []string{"workers"},
				OkStatuses: []int{http.StatusOK},
				Query:      map[string]string{"a": "1", "b": "2"},
			},
		},
		{
			name: "add project key",
			stub: NewServerStub(t).
				WithGetAllEndpoint().
				WithProjectKey("projectKey"),
			params: APICallParams{
				Method:     "GET",
				Path:       []string{"workers"},
				OkStatuses: []int{http.StatusOK},
				ProjectKey: "projectKey",
			},
		},
		{
			name: "add project key amongst query params",
			stub: NewServerStub(t).
				WithGetAllEndpoint().
				WithQueryParam("a", "1").
				WithProjectKey("projectKey"),
			params: APICallParams{
				Method:     "GET",
				Path:       []string{"workers"},
				OkStatuses: []int{http.StatusOK},
				ProjectKey: "projectKey",
				Query:      map[string]string{"a": "1"},
			},
		},
		{
			name: "process response",
			stub: NewServerStub(t).WithGetAllEndpoint().WithWorkers(&model.WorkerDetails{Key: "wk-0"}),
			params: APICallParams{
				Method:     "GET",
				Path:       []string{"workers"},
				OkStatuses: []int{http.StatusOK},
				OnContent: func(content []byte) error {
					var allWorkers struct {
						Workers []model.WorkerDetails `json:"workers"`
					}
					err := json.Unmarshal(content, &allWorkers)
					if err != nil {
						return err
					}

					require.Len(t, allWorkers.Workers, 1)
					assert.Equal(t, "wk-0", allWorkers.Workers[0].Key)

					return nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, token := NewMockWorkerServer(t, tt.stub.WithT(t))

			tt.params.ServerURL = s.BaseUrl()
			tt.params.ServerToken = token

			ctx := tt.ctx
			if ctx == nil {
				ctx = IntFlagMap{}
			}

			err := CallWorkerAPI(ctx, tt.params)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Regexpf(t, tt.wantErr, err.Error(), "got: %+v", err)
			}
		})
	}
}

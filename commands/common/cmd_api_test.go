//go:build test
// +build test

package common

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestCallWorkerAPI_PathEscaping(t *testing.T) {
	tests := []struct {
		name    string
		path    []string
		wantURI string
		wantErr string
	}{
		{name: "ticket case", path: []string{"workers", "../x?y"}, wantURI: "/worker/api/v1/workers/..%2Fx%3Fy?projectKey=p"},
		{
			name:    "slashes",
			path:    []string{"workers", "x/../../../../artifactory/api/repositories/libs-release-local"},
			wantURI: "/worker/api/v1/workers/x%2F..%2F..%2F..%2F..%2Fartifactory%2Fapi%2Frepositories%2Flibs-release-local?projectKey=p",
		},
		{name: "hash", path: []string{"execute", "a#b"}, wantURI: "/worker/api/v1/execute/a%23b?projectKey=p"},
		{name: "plain key", path: []string{"workers", "wk-1"}, wantURI: "/worker/api/v1/workers/wk-1?projectKey=p"},
		{name: "constant path", path: []string{"scaffold", "tsconfig"}, wantURI: "/worker/api/v1/scaffold/tsconfig?projectKey=p"},
		{name: "double dot", path: []string{"workers", ".."}, wantErr: "invalid path element '..' in worker API call"},
		{name: "single dot", path: []string{"workers", "."}, wantErr: "invalid path element '.' in worker API call"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotURI string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotURI = r.RequestURI
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			err := CallWorkerAPI(IntFlagMap{}, APICallParams{
				Method:     http.MethodGet,
				ServerURL:  srv.URL,
				Path:       tt.path,
				ProjectKey: "p",
				OkStatuses: []int{http.StatusOK},
			})

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Empty(t, gotURI, "no request should be sent")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURI, gotURI)
		})
	}
}

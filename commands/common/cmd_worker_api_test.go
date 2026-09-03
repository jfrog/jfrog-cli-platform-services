//go:build test
// +build test

package common

import (
	"net/http"
	"testing"

	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchWorkerDetails(t *testing.T) {
	tests := []struct {
		name       string
		ctx        model.IntFlagProvider
		workerKey  string
		projectKey string
		stub       *ServerStub
		wantErr    string
		want       *model.WorkerDetails
	}{
		{
			name:      "success",
			workerKey: "wk-1",
			stub:      NewServerStub(t).WithGetOneEndpoint().WithWorkers(&model.WorkerDetails{Key: "wk-1"}),
			want:      &model.WorkerDetails{Key: "wk-1"},
		},
		{
			name:      "no error if not found",
			workerKey: "wk-1",
			stub:      NewServerStub(t).WithGetOneEndpoint(),
		},
		{
			name:       "propagate projectKey",
			workerKey:  "wk-2",
			projectKey: "prj-1",
			stub: NewServerStub(t).
				WithGetOneEndpoint().
				WithWorkers(&model.WorkerDetails{Key: "wk-2"}).
				WithProjectKey("prj-1"),
			want: &model.WorkerDetails{Key: "wk-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, token := NewMockWorkerServer(t, tt.stub.WithT(t))

			if tt.ctx == nil {
				tt.ctx = IntFlagMap{}
			}

			got, err := FetchWorkerDetails(tt.ctx, s.BaseUrl(), token, tt.workerKey, tt.projectKey)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Regexp(t, tt.wantErr, err.Error())
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFetchActions(t *testing.T) {
	samples := LoadSampleActions(t)

	tests := []struct {
		name       string
		projectKey string
		stub       *ServerStub
		wantErr    string
	}{
		{
			name: "success",
			stub: NewServerStub(t).WithDefaultActionsMetadataEndpoint(),
		},
		{
			name:       "propagate projectKey",
			projectKey: "prj-1",
			stub: NewServerStub(t).
				WithDefaultActionsMetadataEndpoint().
				WithProjectKey("prj-1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, token := NewMockWorkerServer(t, tt.stub.WithT(t))

			got, err := FetchActions(IntFlagMap{}, s.BaseUrl(), token, tt.projectKey)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Regexp(t, tt.wantErr, err.Error())
				return
			}

			require.NoError(t, err)
			assert.Len(t, got, len(samples))
		})
	}
}

func TestFetchOptions(t *testing.T) {
	samples := LoadSampleOptions(t)
	s, token := NewMockWorkerServer(t, NewServerStub(t).WithOptionsEndpoint().WithT(t))
	got, err := FetchOptions(IntFlagMap{}, s.BaseUrl(), token)
	require.NoError(t, err)
	assert.Equal(t, got, samples)
}

func TestFetchTsConfig(t *testing.T) {
	const validTsConfig = `{"compilerOptions":{"strict":true}}`

	tests := []struct {
		name    string
		stub    *ServerStub
		wantErr string
		want    []byte
	}{
		{
			name: "success",
			stub: NewServerStub(t).WithTsConfigEndpoint(http.StatusOK, validTsConfig),
			want: []byte(validTsConfig),
		},
		{
			name:    "endpoint not found",
			stub:    NewServerStub(t),
			wantErr: `command GET .+/worker/api/v1/scaffold/tsconfig returned an unexpected status code 404`,
		},
		{
			name:    "server error",
			stub:    NewServerStub(t).WithTsConfigEndpoint(http.StatusInternalServerError, ""),
			wantErr: `command GET .+/worker/api/v1/scaffold/tsconfig returned an unexpected status code 500`,
		},
		{
			name:    "invalid json body",
			stub:    NewServerStub(t).WithTsConfigEndpoint(http.StatusOK, "not json"),
			wantErr: `server returned an invalid tsconfig.json`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, token := NewMockWorkerServer(t, tt.stub.WithT(t))

			got, err := FetchTsConfig(IntFlagMap{}, s.BaseUrl(), token)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Regexp(t, tt.wantErr, err.Error())
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

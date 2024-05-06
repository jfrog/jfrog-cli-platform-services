package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/workers-cli/model"
)

func TestListCommand(t *testing.T) {
	tests := []struct {
		name           string
		commandArgs    []string
		token          string
		serverBehavior listServerStubBehavior
		wantErr        error
		assertOutput   func(t *testing.T, content []byte)
	}{
		{
			name: "list",
			serverBehavior: listServerStubBehavior{
				existingWorkers: []*model.WorkerDetails{
					{
						Key:         "wk-0",
						Action:      model.ActionAfterCreate,
						Description: "run wk-0",
						Enabled:     true,
						SourceCode:  "export default async () => ({ 'S': 'OK'})",
					},
				},
			},
			assertOutput: func(t *testing.T, content []byte) {
				assert.Equalf(t, "wk-0,AFTER_CREATE,run wk-0,true", strings.TrimSpace(string(content)), "invalid csv received")
			},
		},
		{
			name:        "list worker of type",
			commandArgs: []string{"AFTER_CREATE"},
			serverBehavior: listServerStubBehavior{
				wantAction: "AFTER_CREATE",
				existingWorkers: []*model.WorkerDetails{
					{
						Key:         "wk-0",
						Action:      model.ActionAfterCreate,
						Description: "run wk-0",
						Enabled:     true,
						SourceCode:  "export default async () => ({ 'S': 'OK'})",
					},
					{
						Key:         "wk-1",
						Action:      model.ActionBeforeDownload,
						Description: "run wk-1",
						Enabled:     true,
						SourceCode:  "export default async () => ({ 'S': 'OK'})",
					},
				},
			},
			assertOutput: func(t *testing.T, content []byte) {
				assert.Equalf(t, "wk-0,AFTER_CREATE,run wk-0,true", strings.TrimSpace(string(content)), "invalid csv received")
			},
		},
		{
			name:        "list for JSON",
			commandArgs: []string{"--" + model.FlagJsonOutput},
			serverBehavior: listServerStubBehavior{
				existingWorkers: []*model.WorkerDetails{
					{
						Key:         "wk-1",
						Action:      model.ActionGenericEvent,
						Description: "run wk-1",
						Enabled:     false,
						SourceCode:  "export default async () => ({ 'S': 'OK'})",
					},
				},
			},
			assertOutput: func(t *testing.T, content []byte) {
				workers := getAllResponse{}
				require.NoError(t, json.Unmarshal(content, &workers))
				assert.Len(t, workers.Workers, 1)
				assert.Equalf(t, "wk-1", workers.Workers[0].Key, "Key mismatch")
				assert.Equalf(t, model.ActionGenericEvent, workers.Workers[0].Action, "Action mismatch")
				assert.Equalf(t, "run wk-1", workers.Workers[0].Description, "Descritption mismatch")
				assert.Equalf(t, false, workers.Workers[0].Enabled, "Enabled mismatch")
				assert.Equalf(t, "export default async () => ({ 'S': 'OK'})", workers.Workers[0].SourceCode, "Source Code mismatch")
			},
		},
		{
			name:        "fails if timeout exceeds",
			commandArgs: []string{"--" + model.FlagTimeout, "500"},
			serverBehavior: listServerStubBehavior{
				waitFor: 5 * time.Second,
			},
			wantErr: errors.New("request timed out after 500ms"),
		},
		{
			name:        "fails if invalid timeout",
			commandArgs: []string{"--" + model.FlagTimeout, "abc"},
			wantErr:     errors.New("invalid timeout provided"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancelCtx := context.WithCancel(context.Background())
			t.Cleanup(cancelCtx)

			runCmd := createCliRunner(t, GetListCommand())

			err := os.Setenv(model.EnvKeyServerUrl, newListServerStub(t, ctx, &tt.serverBehavior))
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.Unsetenv(model.EnvKeyServerUrl)
			})

			if tt.token == "" && tt.serverBehavior.wantBearerToken == "" {
				tt.token = t.Name()
				tt.serverBehavior.wantBearerToken = t.Name()
			}

			err = os.Setenv(model.EnvKeyAccessToken, tt.token)
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.Unsetenv(model.EnvKeyAccessToken)
			})

			var output bytes.Buffer
			cliOut = &output
			t.Cleanup(func() {
				cliOut = os.Stdout
			})

			cmd := append([]string{"worker", "list"}, tt.commandArgs...)

			err = runCmd(cmd...)

			cancelCtx()

			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, tt.wantErr, err.Error())
			}

			if tt.assertOutput != nil {
				tt.assertOutput(t, output.Bytes())
			}
		})
	}
}

var listUrlPattern = regexp.MustCompile(`^/worker/api/v1/workers(/[\S/]+)?$`)

type listServerStubBehavior struct {
	waitFor         time.Duration
	responseStatus  int
	wantBearerToken string
	wantAction      string
	existingWorkers []*model.WorkerDetails
}

type listServerStub struct {
	t        *testing.T
	ctx      context.Context
	behavior *listServerStubBehavior
}

func newListServerStub(t *testing.T, ctx context.Context, behavior *listServerStubBehavior) string {
	stub := listServerStub{t: t, behavior: behavior, ctx: ctx}
	server := httptest.NewUnstartedServer(&stub)
	t.Cleanup(server.Close)
	server.Start()
	return "http:" + "//" + server.Listener.Addr().String()
}

func (s *listServerStub) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	urlMatch := listUrlPattern.FindAllStringSubmatch(req.URL.Path, -1)
	if len(urlMatch) == 0 {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	if s.behavior.waitFor > 0 {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(s.behavior.waitFor):
		}
	}

	// Validate method
	if http.MethodGet != req.Method {
		res.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Validate token
	if req.Header.Get("authorization") != "Bearer "+s.behavior.wantBearerToken {
		res.WriteHeader(http.StatusForbidden)
		return
	}

	if s.behavior.responseStatus > 0 {
		res.WriteHeader(s.behavior.responseStatus)
		return
	}

	var workers []*model.WorkerDetails

	if s.behavior.wantAction == "" {
		workers = s.behavior.existingWorkers
	} else {
		for _, wk := range s.behavior.existingWorkers {
			if wk.Action == s.behavior.wantAction {
				workers = append(workers, wk)
			}
		}
	}

	res.WriteHeader(http.StatusOK)
	_, err := res.Write([]byte(mustJsonMarshal(s.t, getAllResponse{Workers: workers})))
	require.NoError(s.t, err)
}

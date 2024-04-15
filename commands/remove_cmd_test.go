package commands

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/workers-cli/model"
)

func TestRemoveCommand(t *testing.T) {
	tests := []struct {
		name           string
		commandArgs    []string
		token          string
		workerAction   string
		workerName     string
		skipInit       bool
		serverBehavior removeServerStubBehavior
		wantErr        error
	}{
		{
			name:         "undeploy from manifest",
			workerAction: "BEFORE_UPLOAD",
			workerName:   "wk-0",
			serverBehavior: removeServerStubBehavior{
				wantWorkerKey: "wk-0",
			},
		},
		{
			name:        "undeploy from key",
			workerName:  "wk-1",
			skipInit:    true,
			commandArgs: []string{"wk-1"},
			serverBehavior: removeServerStubBehavior{
				wantWorkerKey: "wk-1",
			},
		},
		{
			name:        "fails if timeout exceeds",
			commandArgs: []string{"--" + model.FlagTimeout, "500"},
			serverBehavior: removeServerStubBehavior{
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

			runCmd := createCliRunner(t, GetInitCommand(), GetRemoveCommand())

			_, workerName := prepareWorkerDirForTest(t)
			if tt.workerName != "" {
				workerName = tt.workerName
			}

			if !tt.skipInit {
				workerAction := tt.workerAction
				if workerAction == "" {
					workerAction = "BEFORE_DOWNLOAD"
				}

				err := runCmd("worker", "init", workerAction, workerName)
				require.NoError(t, err)
			}

			err := os.Setenv(model.EnvKeyServerUrl, newRemoveServerStub(t, ctx, &tt.serverBehavior))
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

			cmd := append([]string{"worker", "undeploy"}, tt.commandArgs...)

			err = runCmd(cmd...)

			cancelCtx()

			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, tt.wantErr, err.Error())
			}
		})
	}
}

var removeUrlPattern = regexp.MustCompile(`^/worker/api/v1/workers/([\S/]+)$`)

type removeServerStubBehavior struct {
	waitFor         time.Duration
	responseStatus  int
	wantBearerToken string
	wantWorkerKey   string
}

type removeServerStub struct {
	t        *testing.T
	ctx      context.Context
	behavior *removeServerStubBehavior
}

func newRemoveServerStub(t *testing.T, ctx context.Context, behavior *removeServerStubBehavior) string {
	stub := removeServerStub{t: t, behavior: behavior, ctx: ctx}
	server := httptest.NewUnstartedServer(&stub)
	t.Cleanup(server.Close)
	server.Start()
	return "http:" + "//" + server.Listener.Addr().String()
}

func (s *removeServerStub) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	urlMatch := removeUrlPattern.FindAllStringSubmatch(req.URL.Path, -1)
	if len(urlMatch) == 0 && len(urlMatch[0]) < 2 {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	if s.behavior.wantWorkerKey != "" && s.behavior.wantWorkerKey != urlMatch[0][1] {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	if s.behavior.waitFor > 0 {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(s.behavior.waitFor):
		}
	}

	if req.Method != http.MethodDelete {
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

	res.WriteHeader(http.StatusNoContent)
}

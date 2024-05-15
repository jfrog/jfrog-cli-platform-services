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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func TestListEventCommand(t *testing.T) {
	tests := []struct {
		name           string
		commandArgs    []string
		token          string
		serverBehavior listEventServerStubBehavior
		wantErr        error
		assertOutput   func(t *testing.T, content []byte)
	}{
		{
			name: "list",
			serverBehavior: listEventServerStubBehavior{
				events: []string{"A", "B", "C"},
			},
			assertOutput: func(t *testing.T, content []byte) {
				var events []string
				require.NoError(t, json.Unmarshal(content, &events))
				assert.ElementsMatch(t, []string{"A", "B", "C"}, events)
			},
		},
		{
			name:        "fails if timeout exceeds",
			commandArgs: []string{"--" + model.FlagTimeout, "500"},
			serverBehavior: listEventServerStubBehavior{
				waitFor: 5 * time.Second,
			},
			wantErr: errors.New("request timed out after 500ms"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancelCtx := context.WithCancel(context.Background())
			t.Cleanup(cancelCtx)

			runCmd := createCliRunner(t, GetListEventsCommand())

			err := os.Setenv(model.EnvKeyServerUrl, newListEventServerStub(t, ctx, &tt.serverBehavior))
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

			cmd := append([]string{"worker", "list-event"}, tt.commandArgs...)

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

var listEventUrlPattern = regexp.MustCompile(`^/worker/api/v1/actions$`)

type listEventServerStubBehavior struct {
	waitFor         time.Duration
	responseStatus  int
	wantBearerToken string
	events          []string
}

type listEventServerStub struct {
	t        *testing.T
	ctx      context.Context
	behavior *listEventServerStubBehavior
}

func newListEventServerStub(t *testing.T, ctx context.Context, behavior *listEventServerStubBehavior) string {
	stub := listEventServerStub{t: t, behavior: behavior, ctx: ctx}
	server := httptest.NewUnstartedServer(&stub)
	t.Cleanup(server.Close)
	server.Start()
	return "http:" + "//" + server.Listener.Addr().String()
}

func (s *listEventServerStub) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	urlMatch := listEventUrlPattern.FindAllStringSubmatch(req.URL.Path, -1)
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

	res.WriteHeader(http.StatusOK)

	resBytes, err := json.Marshal(s.behavior.events)
	require.NoError(s.t, err)

	_, err = res.Write(resBytes)
	require.NoError(s.t, err)
}

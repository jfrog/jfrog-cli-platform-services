package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

type dryRunAssertFunc func(t *testing.T, stdOutput []byte, err error, serverBehavior *dryRunServerStubBehavior)

func TestDryRun(t *testing.T) {
	tests := []struct {
		name        string
		commandArgs []string
		assert      dryRunAssertFunc
		// Token to be sent in the request
		token string
		// The server behavior
		serverBehavior *dryRunServerStubBehavior
		// If provided the cliIn will be filled with this content
		stdInput string
		// If provided a temp file will be generated with this content and the file path will be added at the end of the command
		fileInput string
	}{
		{
			name:  "nominal case",
			token: "my-token",
			serverBehavior: &dryRunServerStubBehavior{
				responseStatus: http.StatusOK,
				responseBody: map[string]any{
					"my": "payload",
				},
				requestToken: "my-token",
			},
			commandArgs: []string{mustJsonMarshal(t, map[string]any{"my": "payload"})},
			assert:      assertDryRunSucceed,
		},
		{
			name:  "fails if not OK status",
			token: "invalid-token",
			serverBehavior: &dryRunServerStubBehavior{
				requestToken: "valid-token",
			},
			commandArgs: []string{`{}`},
			assert:      assertDryRunFail("command failed with status %d", http.StatusForbidden),
		},
		{
			name:     "reads from stdin",
			token:    "valid-token",
			stdInput: mustJsonMarshal(t, map[string]any{"my": "request"}),
			serverBehavior: &dryRunServerStubBehavior{
				requestToken:   "valid-token",
				requestBody:    map[string]any{"my": "request"},
				responseBody:   map[string]any{"valid": "response"},
				responseStatus: http.StatusOK,
			},
			commandArgs: []string{"-"},
			assert:      assertDryRunSucceed,
		},
		{
			name:      "reads from file",
			token:     "valid-token",
			fileInput: mustJsonMarshal(t, map[string]any{"my": "file-content"}),
			serverBehavior: &dryRunServerStubBehavior{
				requestToken:   "valid-token",
				requestBody:    map[string]any{"my": "file-content"},
				responseBody:   map[string]any{"valid": "response"},
				responseStatus: http.StatusOK,
			},
			assert: assertDryRunSucceed,
		},
		{
			name:        "fails if invalid json from argument",
			commandArgs: []string{`{"my":`},
			assert:      assertDryRunFail("invalid json payload: unexpected end of JSON input"),
		},
		{
			name:      "fails if invalid json from file argument",
			fileInput: `{"my":`,
			assert:    assertDryRunFail("invalid json payload: unexpected end of JSON input"),
		},
		{
			name:        "fails if invalid json from standard input",
			commandArgs: []string{"-"},
			stdInput:    `{"my":`,
			assert:      assertDryRunFail("unexpected EOF"),
		},
		{
			name:        "fails if missing file",
			commandArgs: []string{"@non-existing-file.json"},
			assert:      assertDryRunFail("open non-existing-file.json: no such file or directory"),
		},
		{
			name:        "fails if timeout exceeds",
			commandArgs: []string{"--" + model.FlagTimeout, "500", `{}`},
			serverBehavior: &dryRunServerStubBehavior{
				waitFor: 5 * time.Second,
			},
			assert: assertDryRunFail("request timed out after 500ms"),
		},
		{
			name:        "fails if invalid timeout",
			commandArgs: []string{"--" + model.FlagTimeout, "abc", `{}`},
			assert:      assertDryRunFail("invalid timeout provided"),
		},
		{
			name:        "fails if empty file path",
			commandArgs: []string{"@"},
			assert:      assertDryRunFail("missing file path"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancelCtx := context.WithCancel(context.Background())
			t.Cleanup(cancelCtx)

			runCmd := createCliRunner(t, GetInitCommand(), GetDryRunCommand())

			_, workerName := prepareWorkerDirForTest(t)

			err := runCmd("worker", "init", "BEFORE_DOWNLOAD", workerName)
			require.NoError(t, err)

			serverResponseStubs := map[string]*dryRunServerStubBehavior{}
			if tt.serverBehavior != nil {
				serverResponseStubs[workerName] = tt.serverBehavior
			}

			err = os.Setenv(model.EnvKeyServerUrl, newDryRunServerStub(t, ctx, serverResponseStubs))
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.Unsetenv(model.EnvKeyServerUrl)
			})

			err = os.Setenv(model.EnvKeyAccessToken, tt.token)
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.Unsetenv(model.EnvKeyAccessToken)
			})

			err = os.Setenv(model.EnvKeySecretsPassword, secretPassword)
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.Unsetenv(model.EnvKeySecretsPassword)
			})

			if tt.stdInput != "" {
				cliIn = bytes.NewReader([]byte(tt.stdInput))
				t.Cleanup(func() {
					cliIn = os.Stdin
				})
			}

			if tt.fileInput != "" {
				tt.commandArgs = append(tt.commandArgs, "@"+createTempFileWithContent(t, tt.fileInput))
			}

			var output bytes.Buffer

			cliOut = &output
			t.Cleanup(func() {
				cliOut = os.Stdout
			})

			cmd := append([]string{"worker", "dry-run"}, tt.commandArgs...)

			err = runCmd(cmd...)

			cancelCtx()

			tt.assert(t, output.Bytes(), err, tt.serverBehavior)
		})
	}
}

func assertDryRunSucceed(t *testing.T, output []byte, err error, serverBehavior *dryRunServerStubBehavior) {
	require.NoError(t, err)

	outputData := map[string]any{}

	err = json.Unmarshal(output, &outputData)
	require.NoError(t, err)

	assert.Equal(t, serverBehavior.responseBody, outputData)
}

func assertDryRunFail(errorMessage string, errorMessageArgs ...any) dryRunAssertFunc {
	return func(t *testing.T, stdOutput []byte, err error, serverResponse *dryRunServerStubBehavior) {
		require.Error(t, err)
		assert.EqualError(t, err, fmt.Sprintf(errorMessage, errorMessageArgs...))
	}
}

var dryRunUrlPattern = regexp.MustCompile(`^/worker/api/v1/test/([\S/]+)$`)

type dryRunServerStubBehavior struct {
	waitFor        time.Duration
	responseStatus int
	responseBody   map[string]any
	requestToken   string
	requestBody    map[string]any
}

type dryRunServerStub struct {
	t     *testing.T
	ctx   context.Context
	stubs map[string]*dryRunServerStubBehavior
}

func newDryRunServerStub(t *testing.T, ctx context.Context, responseStubs map[string]*dryRunServerStubBehavior) string {
	stub := dryRunServerStub{stubs: responseStubs, ctx: ctx}
	server := httptest.NewUnstartedServer(&stub)
	t.Cleanup(server.Close)
	server.Start()
	return "http:" + "//" + server.Listener.Addr().String()
}

func (s *dryRunServerStub) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	matches := dryRunUrlPattern.FindAllStringSubmatch(req.URL.Path, -1)
	if len(matches) == 0 || len(matches[0][1]) < 1 {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	if req.Header.Get("content-type") != "application/json" {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	workerName := matches[0][1]

	behavior, exists := s.stubs[workerName]
	if !exists {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	if behavior.waitFor > 0 {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(behavior.waitFor):
		}
	}

	// Validate token
	if req.Header.Get("authorization") != "Bearer "+behavior.requestToken {
		res.WriteHeader(http.StatusForbidden)
		return
	}

	// Validate body if requested
	if behavior.requestBody != nil {
		wantData, checkRequestData := behavior.responseBody["data"]

		if checkRequestData {
			gotData, err := io.ReadAll(req.Body)
			if err != nil {
				s.t.Logf("Read request body error: %+v", err)
				res.WriteHeader(http.StatusInternalServerError)
				return
			}

			decodedData := map[string]any{}
			err = json.Unmarshal(gotData, &decodedData)
			if err != nil {
				s.t.Logf("Unmarshall request body error: %+v", err)
				res.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !reflect.DeepEqual(wantData, decodedData) {
				res.WriteHeader(http.StatusBadRequest)
				return
			}
		}
	}

	bodyBytes, err := json.Marshal(behavior.responseBody)
	if err != nil {
		s.t.Logf("Marshall error: %+v", err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(behavior.responseStatus)
	_, err = res.Write(bodyBytes)
	if err != nil {
		s.t.Logf("Write error: %+v", err)
		res.WriteHeader(http.StatusInternalServerError)
	}
}

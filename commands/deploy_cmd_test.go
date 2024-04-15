package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func TestDeployCommand(t *testing.T) {
	tests := []struct {
		name           string
		commandArgs    []string
		token          string
		workerAction   string
		workerName     string
		serverBehavior deployServerStubBehavior
		wantErr        error
		patchManifest  func(mf *model.Manifest)
	}{
		{
			name:         "create",
			workerAction: "BEFORE_UPLOAD",
			workerName:   "wk-0",
			serverBehavior: deployServerStubBehavior{
				wantMethods: []string{http.MethodGet, http.MethodPost},
				wantRequestBody: getExpectedDeployRequestForAction(
					t,
					"wk-0",
					"BEFORE_UPLOAD",
					&model.Secret{Key: "sec-1", Value: "val-1"},
					&model.Secret{Key: "sec-2", Value: "val-2"},
				),
			},
			patchManifest: func(mf *model.Manifest) {
				mf.Secrets = model.Secrets{
					"sec-1": mustEncryptSecret(t, "val-1"),
					"sec-2": mustEncryptSecret(t, "val-2"),
				}
			},
		},
		{
			name:         "update",
			workerAction: "GENERIC_EVENT",
			workerName:   "wk-1",
			serverBehavior: deployServerStubBehavior{
				wantMethods:     []string{http.MethodGet, http.MethodPut},
				wantRequestBody: getExpectedDeployRequestForAction(t, "wk-1", "GENERIC_EVENT"),
				existingWorkers: map[string]*model.WorkerDetails{
					"wk-1": {},
				},
			},
		},
		{
			name:         "update with removed secrets",
			workerAction: "AFTER_MOVE",
			workerName:   "wk-2",
			serverBehavior: deployServerStubBehavior{
				wantMethods: []string{http.MethodGet, http.MethodPut},
				wantRequestBody: getExpectedDeployRequestForAction(
					t,
					"wk-2",
					"AFTER_MOVE",
					&model.Secret{Key: "sec-1", MarkedForRemoval: true},
					&model.Secret{Key: "sec-1", Value: "val-1"},
					&model.Secret{Key: "sec-2", MarkedForRemoval: true},
				),
				existingWorkers: map[string]*model.WorkerDetails{
					"wk-2": {
						Secrets: []*model.Secret{
							{Key: "sec-1"}, {Key: "sec-2"},
						},
					},
				},
			},
			patchManifest: func(mf *model.Manifest) {
				mf.Secrets = model.Secrets{
					"sec-1": mustEncryptSecret(t, "val-1"),
				}
			},
		},
		{
			name:        "fails if timeout exceeds",
			commandArgs: []string{"--" + model.FlagTimeout, "500"},
			serverBehavior: deployServerStubBehavior{
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

			runCmd := createCliRunner(t, GetInitCommand(), GetDeployCommand())

			_, workerName := prepareWorkerDirForTest(t)
			if tt.workerName != "" {
				workerName = tt.workerName
			}

			workerAction := tt.workerAction
			if workerAction == "" {
				workerAction = "BEFORE_DOWNLOAD"
			}

			err := runCmd("worker", "init", workerAction, workerName)
			require.NoError(t, err)

			if tt.patchManifest != nil {
				patchManifest(t, tt.patchManifest)
			}

			err = os.Setenv(model.EnvKeyServerUrl, newDeployServerStub(t, ctx, &tt.serverBehavior))
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.Unsetenv(model.EnvKeyServerUrl)
			})

			err = os.Setenv(model.EnvKeySecretsPassword, secretPassword)
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.Unsetenv(model.EnvKeySecretsPassword)
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

			cmd := append([]string{"worker", "deploy"}, tt.commandArgs...)

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

var deployUrlPattern = regexp.MustCompile(`^/worker/api/v1/workers(/[\S/]+)?$`)

type deployServerStubBehavior struct {
	waitFor         time.Duration
	responseStatus  int
	wantBearerToken string
	wantRequestBody *deployRequest
	wantMethods     []string
	existingWorkers map[string]*model.WorkerDetails
}

type deployServerStub struct {
	t        *testing.T
	ctx      context.Context
	behavior *deployServerStubBehavior
}

func newDeployServerStub(t *testing.T, ctx context.Context, behavior *deployServerStubBehavior) string {
	stub := deployServerStub{t: t, behavior: behavior, ctx: ctx}
	server := httptest.NewUnstartedServer(&stub)
	t.Cleanup(server.Close)
	server.Start()
	return "http:" + "//" + server.Listener.Addr().String()
}

func (s *deployServerStub) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	urlMatch := deployUrlPattern.FindAllStringSubmatch(req.URL.Path, -1)
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
	var methodValid bool
	for _, wantMethod := range s.behavior.wantMethods {
		if methodValid = wantMethod == req.Method; methodValid {
			break
		}
	}

	if !methodValid {
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

	if http.MethodGet != req.Method {
		if req.Header.Get("content-type") != "application/json" {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		// Validate body if requested
		if s.behavior.wantRequestBody != nil {
			gotData, err := io.ReadAll(req.Body)
			if err != nil {
				s.t.Logf("Read request body error: %+v", err)
				res.WriteHeader(http.StatusInternalServerError)
				return
			}

			gotRequestBody := deployRequest{}
			err = json.Unmarshal(gotData, &gotRequestBody)
			if err != nil {
				s.t.Logf("Unmarshall request body error: %+v", err)
				res.WriteHeader(http.StatusInternalServerError)
				return
			}

			assertDeployRequestEquals(s.t, s.behavior.wantRequestBody, &gotRequestBody)
		}
	}

	if http.MethodGet == req.Method {
		var workerKey string

		if len(urlMatch[0]) < 1 {
			res.WriteHeader(http.StatusNotFound)
			return
		} else {
			workerKey = urlMatch[0][1][1:]
		}

		workerDetails, workerExists := s.behavior.existingWorkers[workerKey]
		if !workerExists {
			res.WriteHeader(http.StatusNotFound)
			return
		}

		res.WriteHeader(http.StatusOK)
		_, err := res.Write([]byte(mustJsonMarshal(s.t, workerDetails)))
		require.NoError(s.t, err)
		return
	}

	// Assume updated or created
	if http.MethodPut == req.Method {
		res.WriteHeader(http.StatusNoContent)
		return
	} else if http.MethodPost == req.Method {
		res.WriteHeader(http.StatusCreated)
		return
	}

	res.WriteHeader(http.StatusMethodNotAllowed)
}

func assertDeployRequestEquals(t require.TestingT, want, got *deployRequest) {
	assert.Equalf(t, want.Key, got.Key, "Key mismatch")
	assert.Equalf(t, want.Description, got.Description, "Description mismatch")
	assert.Equalf(t, want.Enabled, got.Enabled, "Enabled mismatch")
	assert.Equalf(t, want.SourceCode, got.SourceCode, "SourceCode mismatch")
	assert.Equalf(t, want.Action, got.Action, "Action mismatch")
	assert.Equalf(t, want.FilterCriteria, got.FilterCriteria, "FilterCriteria mismatch")

	// Equals does not work for secrets as they are proto objects
	assert.Equalf(t, len(want.Secrets), len(got.Secrets), "Secrets length mismatch")
	var wantSecrets, gotSecrets []string
	for _, s := range want.Secrets {
		wantSecrets = append(wantSecrets, fmt.Sprintf("%s:%s:%v", s.Key, s.Value, s.MarkedForRemoval))
	}
	for _, s := range got.Secrets {
		gotSecrets = append(gotSecrets, fmt.Sprintf("%s:%s:%v", s.Key, s.Value, s.MarkedForRemoval))
	}
	assert.ElementsMatchf(t, wantSecrets, gotSecrets, "Secrets mismatch")
}

func getExpectedDeployRequestForAction(t require.TestingT, workerName, actionName string, secrets ...*model.Secret) *deployRequest {
	r := &deployRequest{
		Key:         workerName,
		Description: "Run a script on " + actionName,
		Enabled:     false,
		SourceCode:  cleanImports(getActionSourceCode(t, actionName)),
		Action:      actionName,
		Secrets:     secrets,
	}

	if model.ActionNeedsCriteria(actionName) {
		r.FilterCriteria = model.FilterCriteria{
			ArtifactFilterCriteria: model.ArtifactFilterCriteria{
				RepoKeys: []string{"example-repo-local"},
			},
		}
	}

	return r
}

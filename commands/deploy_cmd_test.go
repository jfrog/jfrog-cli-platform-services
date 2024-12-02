//go:build test
// +build test

package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func TestDeployCommand(t *testing.T) {
	actionsMeta := common.LoadSampleActions(t)

	tests := []struct {
		name           string
		commandArgs    []string
		token          string
		workerAction   string
		workerName     string
		serverBehavior *common.ServerStub
		wantErr        error
		patchManifest  func(mf *model.Manifest)
	}{
		{
			name:         "create",
			workerAction: "BEFORE_UPLOAD",
			workerName:   "wk-0",
			serverBehavior: common.NewServerStub(t).
				WithGetOneEndpoint().
				WithCreateEndpoint(
					expectDeployRequest(
						actionsMeta,
						"wk-0",
						"BEFORE_UPLOAD",
						"",
						&model.Secret{Key: "sec-1", Value: "val-1"},
						&model.Secret{Key: "sec-2", Value: "val-2"},
					),
				),
			patchManifest: func(mf *model.Manifest) {
				mf.Secrets = model.Secrets{
					"sec-1": common.MustEncryptSecret(t, "val-1"),
					"sec-2": common.MustEncryptSecret(t, "val-2"),
				}
			},
		},
		{
			name:         "update",
			workerAction: "GENERIC_EVENT",
			workerName:   "wk-1",
			serverBehavior: common.NewServerStub(t).
				WithGetOneEndpoint().
				WithUpdateEndpoint(
					expectDeployRequest(actionsMeta, "wk-1", "GENERIC_EVENT", ""),
				).
				WithWorkers(&model.WorkerDetails{
					Key: "wk-1",
				}),
		},
		{
			name:         "update with removed secrets",
			workerAction: "AFTER_MOVE",
			workerName:   "wk-2",
			serverBehavior: common.NewServerStub(t).
				WithGetOneEndpoint().
				WithUpdateEndpoint(
					expectDeployRequest(
						actionsMeta,
						"wk-2",
						"AFTER_MOVE",
						"",
						&model.Secret{Key: "sec-1", MarkedForRemoval: true},
						&model.Secret{Key: "sec-1", Value: "val-1"},
						&model.Secret{Key: "sec-2", MarkedForRemoval: true},
					),
				).
				WithWorkers(&model.WorkerDetails{
					Key: "wk-2",
					Secrets: []*model.Secret{
						{Key: "sec-1"}, {Key: "sec-2"},
					},
				}),
			patchManifest: func(mf *model.Manifest) {
				mf.Secrets = model.Secrets{
					"sec-1": common.MustEncryptSecret(t, "val-1"),
				}
			},
		},
		{
			name:         "create with project key",
			workerAction: "GENERIC_EVENT",
			workerName:   "wk-1",
			serverBehavior: common.NewServerStub(t).
				WithGetOneEndpoint().
				WithCreateEndpoint(
					expectDeployRequest(actionsMeta, "wk-1", "GENERIC_EVENT", "proj-1"),
				),
			patchManifest: func(mf *model.Manifest) {
				mf.ProjectKey = "proj-1"
			},
		},
		{
			name:        "fails if timeout exceeds",
			commandArgs: []string{"--" + model.FlagTimeout, "500"},
			serverBehavior: common.NewServerStub(t).
				WithDelay(1 * time.Second).
				WithCreateEndpoint(nil),
			wantErr: errors.New("request timed out after 500ms"),
		},
		{
			name:           "fails if invalid timeout",
			serverBehavior: common.NewServerStub(t),
			commandArgs:    []string{"--" + model.FlagTimeout, "abc"},
			wantErr:        errors.New("invalid timeout provided"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			common.NewMockWorkerServer(t, tt.serverBehavior.WithT(t).WithDefaultActionsMetadataEndpoint())

			runCmd := common.CreateCliRunner(t, GetInitCommand(), GetDeployCommand())

			_, workerName := common.PrepareWorkerDirForTest(t)
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
				common.PatchManifest(t, tt.patchManifest)
			}

			cmd := append([]string{"worker", "deploy"}, tt.commandArgs...)

			err = runCmd(cmd...)

			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, tt.wantErr, err.Error())
			}
		})
	}
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

func expectDeployRequest(actionsMeta common.ActionsMetadata, workerName, actionName, projectKey string, secrets ...*model.Secret) common.BodyValidator {
	return func(t require.TestingT, body []byte) {
		want := getExpectedDeployRequestForAction(t, actionsMeta, workerName, actionName, projectKey, secrets...)
		got := &deployRequest{}
		err := json.Unmarshal(body, got)
		require.NoError(t, err)
		assertDeployRequestEquals(t, want, got)
	}
}

func getExpectedDeployRequestForAction(
	t require.TestingT,
	actionsMeta common.ActionsMetadata,
	workerName, actionName, projectKey string,
	secrets ...*model.Secret,
) *deployRequest {
	r := &deployRequest{
		Key:         workerName,
		Description: "Run a script on " + actionName,
		Enabled:     false,
		SourceCode: common.CleanImports(common.GenerateFromSamples(
			t,
			templates,
			actionName,
			workerName,
			"",
			"worker.ts_template",
		)),
		Action:     actionName,
		Secrets:    secrets,
		ProjectKey: projectKey,
	}

	actionMeta, err := actionsMeta.FindAction(actionName)
	require.NoError(t, err)

	if actionMeta.MandatoryFilter && actionMeta.FilterType == model.FilterTypeRepo {
		r.FilterCriteria = model.FilterCriteria{
			ArtifactFilterCriteria: model.ArtifactFilterCriteria{
				RepoKeys: []string{"example-repo-local"},
			},
		}
	}

	return r
}

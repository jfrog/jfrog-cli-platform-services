//go:build itest

package commands

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/itests/infra"
	"github.com/jfrog/jfrog-cli-platform-services/model"
)

type deployTestCase struct {
	name          string
	only          bool
	skip          bool
	wantErr       error
	workerKey     string
	commandArgs   []string
	initWorkers   []*model.WorkerDetails
	patchManifest func(mf *model.Manifest)
}

func TestDeployCommand(t *testing.T) {
	infra.RunITests([]infra.TestDefinition{
		deployTestSpec(deployTestCase{
			name:      "create",
			workerKey: "wk-0",
		}),
		deployTestSpec(deployTestCase{
			name:      "update",
			workerKey: "wk-1",
			initWorkers: []*model.WorkerDetails{
				{
					Key:         "wk-1",
					Description: "My worker",
					Enabled:     true,
					SourceCode:  `export default async function() { return { "status": "OK" } }`,
					Action:      model.ActionGenericEvent,
				},
			},
		}),
		deployTestSpec(deployTestCase{
			name:      "update with secrets",
			workerKey: "wk-3",
			initWorkers: []*model.WorkerDetails{
				{
					Key:         "wk-3",
					Description: "My worker",
					Enabled:     true,
					SourceCode:  `export default async function() { return { "status": "OK" } }`,
					Action:      model.ActionGenericEvent,
					Secrets: []*model.Secret{
						{Key: "sec-1", Value: "val-1"}, {Key: "sec-2", Value: "val-2"},
					},
				},
			},
			patchManifest: func(mf *model.Manifest) {
				mf.Secrets = model.Secrets{
					"sec-3": infra.MustEncryptSecret(t, "val-3"),
				}
			},
		}),
		deployTestSpec(deployTestCase{
			name:        "fails if invalid timeout",
			commandArgs: []string{"--" + model.FlagTimeout, "abc"},
			wantErr:     errors.New("invalid timeout provided"),
		}),
	}, t)
}

func deployTestSpec(tc deployTestCase) infra.TestDefinition {
	return infra.TestDefinition{
		Name: tc.name,
		Only: tc.only,
		Skip: tc.skip,
		Test: func(it *infra.Test) {
			for _, initialWorker := range tc.initWorkers {
				it.CreateWorker(initialWorker)
				it.Cleanup(func() {
					it.DeleteWorker(initialWorker.Key)
				})
			}

			_, workerName := it.PrepareWorkerTestDir()
			if tc.workerKey != "" {
				workerName = tc.workerKey
			}

			err := it.RunCommand("worker", "init", "GENERIC_EVENT", workerName)
			require.NoError(it, err)

			if tc.patchManifest != nil {
				infra.PatchManifest(it, tc.patchManifest)
			}

			infra.AddSecretPasswordToEnv(it)

			cmd := append([]string{"worker", "deploy"}, tc.commandArgs...)

			err = it.RunCommand(cmd...)

			if err == nil {
				// We make sure to undeploy our worker
				it.Cleanup(func() {
					it.DeleteWorker(workerName)
				})
			}

			if tc.wantErr == nil {
				require.NoError(it, err)

				mf, err := model.ReadManifest()
				require.NoError(it, err)

				require.NoError(it, mf.DecryptSecrets())

				assertWorkerDeployed(it, mf)
			} else {
				assert.EqualError(it, err, tc.wantErr.Error())
			}
		},
	}
}

func assertWorkerDeployed(it *infra.Test, mf *model.Manifest) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), 3*time.Second)
	it.Cleanup(cancelCtx)

	deployed := model.WorkerDetails{}

	it.NewHttpRequestWithContext(ctx).
		WithAccessToken().
		Get("/worker/api/v1/workers/" + mf.Name).
		Do().
		IsOk().
		AsObject(&deployed)

	assert.Equalf(it, mf.Name, deployed.Key, "Key mismatch")
	assert.Equalf(it, mf.Action, deployed.Action, "Action mismatch")
	assert.Equalf(it, mf.Description, deployed.Description, "Description mismatch")
	assert.Equalf(it, mf.Enabled, deployed.Enabled, "Enabled mismatch")

	sourceCode, err := mf.ReadSourceCode()
	require.NoError(it, err)
	assert.Equalf(it, sourceCode, deployed.SourceCode, "SourceCode mismatch")

	require.Equalf(it, len(mf.Secrets), len(deployed.Secrets), "Secrets length mismatch")
	for _, deployedSecret := range deployed.Secrets {
		_, secretShouldHaveBeenDeployed := mf.Secrets[deployedSecret.Key]
		assert.Truef(it, secretShouldHaveBeenDeployed, "Invalid deployed secret %s", deployedSecret)
		infra.AssertSecretValueFromServer(it, mf.Name, deployedSecret.Key, mf.Secrets[deployedSecret.Key])
	}
}

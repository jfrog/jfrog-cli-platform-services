//go:build itest

package commands

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/jfrog/jfrog-cli-platform-services/test/infra"
)

type removeTestCase struct {
	name        string
	only        bool
	skip        bool
	wantErr     error
	workerKey   string
	commandArgs []string
}

func TestRemoveCommand(t *testing.T) {
	infra.RunITests([]infra.TestDefinition{
		removeTestSpec(removeTestCase{
			name:      "undeploy from manifest",
			workerKey: "wk-1",
		}),
		removeTestSpec(removeTestCase{
			name:        "undeploy with key",
			workerKey:   "wk-1",
			commandArgs: []string{"wk-1"},
		}),
		removeTestSpec(removeTestCase{
			name:        "fails if invalid timeout",
			commandArgs: []string{"--" + model.FlagTimeout, "abc"},
			wantErr:     errors.New("invalid timeout provided"),
		}),
	}, t)
}

func removeTestSpec(tc removeTestCase) infra.TestDefinition {
	return infra.TestDefinition{
		Name: tc.name,
		Only: tc.only,
		Skip: tc.skip,
		Test: func(it *infra.Test) {
			_, workerName := it.PrepareWorkerTestDir()
			if tc.workerKey != "" {
				workerName = tc.workerKey
			}

			err := it.RunCommand(infra.AppName, "init", "GENERIC_EVENT", workerName)
			require.NoError(it, err)

			err = it.RunCommand(infra.AppName, "deploy")
			require.NoError(it, err)
			if err == nil {
				// We make sure to undeploy our worker
				it.Cleanup(func() {
					it.DeleteWorker(workerName)
				})
			}

			cmd := append([]string{infra.AppName, "undeploy"}, tc.commandArgs...)
			err = it.RunCommand(cmd...)

			if tc.wantErr == nil {
				require.NoError(it, err)

				mf, err := model.ReadManifest()
				require.NoError(it, err)

				assertWorkerRemoved(it, mf)
			} else {
				assert.EqualError(it, err, tc.wantErr.Error())
			}
		},
	}
}

func assertWorkerRemoved(it *infra.Test, mf *model.Manifest) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), 3*time.Second)
	it.Cleanup(cancelCtx)

	it.NewHttpRequestWithContext(ctx).
		WithAccessToken().
		Get("/worker/api/v1/workers/" + mf.Name).
		Do().
		IsNotFound()
}

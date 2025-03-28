//go:build itest

package commands

import (
	"encoding/json"
	"testing"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"
	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/jfrog/jfrog-cli-platform-services/test/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShowExecutionHistory(t *testing.T) {
	tests := []struct {
		name string
		test func(it *infra.Test, workerKey string)
	}{
		{
			name: "nominal",
			test: testShowExecutionHistory(false, 2),
		},
		{
			name: "with test runs",
			test: testShowExecutionHistory(true, 3),
		},
	}

	infra.RunITests([]infra.TestDefinition{
		{
			Name:          "Execution History",
			CaptureOutput: true,
			Test: func(it *infra.Test) {
				dir, workerKey := it.PrepareWorkerTestDir()

				err := it.RunCommand(infra.AppName, "init", "GENERIC_EVENT", workerKey)
				require.NoError(it, err)

				common.PatchManifest(it, func(mf *model.Manifest) {
					mf.Enabled = true
					mf.Debug = true
				}, dir)

				err = it.RunCommand(infra.AppName, "deploy")
				require.NoError(it, err)
				if err == nil {
					// We make sure to undeploy our worker
					it.Cleanup(func() {
						it.DeleteWorker(workerKey)
					})
				}

				payload := map[string]any{"my": "payload"}

				it.ExecuteWorker(workerKey, payload)
				it.ExecuteWorker(workerKey, payload)
				it.TestRunWorker(
					workerKey,
					"worker",
					"GENERIC_EVENT",
					`export default async () => ({"message":"OK"});`,
					payload,
					true,
				)

				for _, tt := range tests {
					it.Run(tt.name, func(t *infra.Test) {
						it.ResetIO()
						tt.test(t, workerKey)
					})
				}
			},
		},
	}, t)
}

func testShowExecutionHistory(showTestRun bool, wantEntries int) func(it *infra.Test, workerKey string) {
	return func(it *infra.Test, workerKey string) {
		cmd := []string{infra.AppName, "execution-history"}

		if showTestRun {
			cmd = append(cmd, "--with-test-runs")
		}

		err := it.RunCommand(cmd...)
		require.NoError(it, err)

		var content any

		err = json.Unmarshal(it.CapturedOutput(), &content)
		require.NoError(it, err)

		assert.Len(it, content, wantEntries)
	}
}

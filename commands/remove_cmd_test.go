//go:build test
// +build test

package commands

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func TestRemoveCommand(t *testing.T) {
	tests := []struct {
		name           string
		commandArgs    []string
		workerAction   string
		workerName     string
		patchManifest  func(mf *model.Manifest)
		serverBehavior *common.ServerStub
		assert         common.AssertOutputFunc
	}{
		{
			name:         "undeploy from manifest",
			workerAction: "BEFORE_UPLOAD",
			workerName:   "wk-0",
			serverBehavior: common.NewServerStub(t).
				WithWorkers(&model.WorkerDetails{Key: "wk-0"}).
				WithDeleteEndpoint(),
		},
		{
			name:        "undeploy from key",
			workerName:  "wk-1",
			commandArgs: []string{"wk-1"},
			serverBehavior: common.NewServerStub(t).
				WithWorkers(&model.WorkerDetails{Key: "wk-1"}).
				WithDeleteEndpoint(),
		},
		{
			name:        "fails if timeout exceeds",
			commandArgs: []string{"--" + model.FlagTimeout, "500"},
			serverBehavior: common.NewServerStub(t).
				WithDelay(2 * time.Second).
				WithDeleteEndpoint(),
			assert: common.AssertOutputError("request timed out after 500ms"),
		},
		{
			name:           "fails if invalid timeout",
			commandArgs:    []string{"--" + model.FlagTimeout, "abc"},
			serverBehavior: common.NewServerStub(t).WithDefaultActionsMetadataEndpoint(),
			assert:         common.AssertOutputError("invalid timeout provided"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.serverBehavior != nil {
				common.NewMockWorkerServer(t, tt.serverBehavior.WithT(t).WithDefaultActionsMetadataEndpoint())
			}

			runCmd := common.CreateCliRunner(t, GetInitCommand(), GetRemoveCommand())

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

			var output bytes.Buffer
			common.SetCliOut(&output)
			t.Cleanup(func() {
				common.SetCliOut(os.Stdout)
			})

			cmd := append([]string{"worker", "undeploy"}, tt.commandArgs...)

			err = runCmd(cmd...)

			if tt.assert == nil {
				assert.NoError(t, err)
			} else {
				tt.assert(t, output.Bytes(), err)
			}
		})
	}
}

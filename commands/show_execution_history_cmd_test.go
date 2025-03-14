//go:build test
// +build test

package commands

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func TestExecutionHistory(t *testing.T) {
	entry := &common.ExecutionHistoryEntryStub{
		Start:   time.Now(),
		End:     time.Now().Add(5 * time.Second),
		TestRun: false,
		Result: common.ExecutionHistoryResultEntryStub{
			Result: "OK",
			Logs:   "not a test run",
		},
	}
	testRunEntry := &common.ExecutionHistoryEntryStub{
		Start:   time.Now().Add(-24 * time.Hour),
		End:     time.Now().Add(-23 * time.Hour),
		TestRun: true,
		Result: common.ExecutionHistoryResultEntryStub{
			Result: "KO",
			Logs:   "test run",
		},
	}

	workerHistory := common.ExecutionHistoryStub{entry, testRunEntry}

	tests := []struct {
		name          string
		commandArgs   []string
		initExtraArgs []string
		assert        common.AssertOutputFunc
		action        string
		workerKey     string
		// The server behavior
		serverStub    *common.ServerStub
		patchManifest func(mf *model.Manifest)
	}{
		{
			name:      "show execution history",
			workerKey: "my-worker",
			serverStub: common.NewServerStub(t).
				WithWorkerExecutionHistory("my-worker", workerHistory).
				WithQueryParam("workerKey", "my-worker", common.EndpointExecutionHistory).
				WithGetExecutionHistoryEndpoint(),
			commandArgs: []string{"my-worker"},
			assert:      common.AssertOutputJson(common.ExecutionHistoryStub{entry}),
		},
		{
			name:      "show execution history with test runs",
			workerKey: "my-worker",
			serverStub: common.NewServerStub(t).
				WithWorkerExecutionHistory("my-worker", workerHistory).
				WithQueryParam("workerKey", "my-worker", common.EndpointExecutionHistory).
				WithGetExecutionHistoryEndpoint(),
			commandArgs: []string{"--with-test-runs", "my-worker"},
			assert:      common.AssertOutputJson(workerHistory),
		},
		{
			name:      "should read workerKey and projectKey from manifest",
			workerKey: "my-worker",
			serverStub: common.NewServerStub(t).
				WithWorkerExecutionHistory("my-worker", workerHistory).
				WithProjectKey("my-project").
				WithQueryParam("workerKey", "my-worker", common.EndpointExecutionHistory).
				WithGetExecutionHistoryEndpoint(),
			initExtraArgs: []string{"--" + model.FlagProjectKey, "my-project"},
			commandArgs:   []string{},
			assert:        common.AssertOutputJson(common.ExecutionHistoryStub{entry}),
		},
		{
			name: "fails if not OK status",
			serverStub: common.NewServerStub(t).
				WithToken("invalid-token").
				WithWorkerExecutionHistory("a-worker", workerHistory).
				WithQueryParam("workerKey", "a-worker", common.EndpointExecutionHistory).
				WithGetExecutionHistoryEndpoint(),
			commandArgs: []string{"a-worker"},
			assert:      common.AssertOutputErrorRegexp(`command.+returned\san\sunexpected\sstatus\scode\s403`),
		},
		{
			name:        "fails if timeout exceeds",
			commandArgs: []string{"--" + model.FlagTimeout, "500"},
			serverStub:  common.NewServerStub(t).WithDelay(2 * time.Second).WithGetExecutionHistoryEndpoint(),
			assert:      common.AssertOutputError("request timed out after 500ms"),
		},
		{
			name:        "fails if invalid timeout",
			commandArgs: []string{"--" + model.FlagTimeout, "abc", `{}`},
			assert:      common.AssertOutputError("invalid timeout provided"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCmd := common.CreateCliRunner(t, GetInitCommand(), GetShowExecutionHistoryCommand())

			_, workerName := common.PrepareWorkerDirForTest(t)

			if tt.workerKey != "" {
				workerName = tt.workerKey
			}

			action := "GENERIC_EVENT"
			if tt.action != "" {
				action = tt.action
			}

			if tt.serverStub == nil {
				tt.serverStub = common.NewServerStub(t)
			}

			common.NewMockWorkerServer(t,
				tt.serverStub.
					WithT(t).
					WithDefaultActionsMetadataEndpoint(),
			)

			initCmd := append([]string{"worker", "init"}, tt.initExtraArgs...)
			err := runCmd(append(initCmd, action, workerName)...)
			if err != nil {
				tt.assert(t, nil, err)
				return
			}

			if tt.patchManifest != nil {
				common.PatchManifest(t, tt.patchManifest)
			}

			var output bytes.Buffer

			common.SetCliOut(&output)
			t.Cleanup(func() {
				common.SetCliOut(os.Stdout)
			})

			cmd := append([]string{"worker", "execution-history"}, tt.commandArgs...)

			err = runCmd(cmd...)

			tt.assert(t, output.Bytes(), err)
		})
	}
}

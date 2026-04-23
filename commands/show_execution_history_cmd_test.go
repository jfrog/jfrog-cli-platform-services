//go:build test
// +build test

package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
	"github.com/jfrog/jfrog-cli-platform-services/commands/common"
	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func testExecutionHistoryWorkerHistory(t *testing.T) (*common.ExecutionHistoryEntryStub, common.ExecutionHistoryStub) {
	entry := &common.ExecutionHistoryEntryStub{
		Start:   time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		End:     time.Date(2024, 1, 15, 10, 0, 5, 0, time.UTC),
		TestRun: false,
		Result: common.ExecutionHistoryResultEntryStub{
			Result: "OK",
			Logs:   "execution succeeded",
		},
	}
	return entry, common.ExecutionHistoryStub{entry}
}

const testExecHistoryWorkerKey = "my-format-test-worker"

func setupExecutionHistoryFormatTest(t *testing.T, workerHistory common.ExecutionHistoryStub) func(args ...string) error {
	serverStub := common.NewServerStub(t).
		WithWorkerExecutionHistory(testExecHistoryWorkerKey, workerHistory).
		WithQueryParam("workerKey", testExecHistoryWorkerKey, common.EndpointExecutionHistory).
		WithGetExecutionHistoryEndpoint()
	common.NewMockWorkerServer(t, serverStub.WithDefaultActionsMetadataEndpoint())

	runCmd := common.CreateCliRunner(t, GetInitCommand(), GetShowExecutionHistoryCommand())
	common.PrepareWorkerDirForTest(t)
	require.NoError(t, runCmd("worker", "init", "GENERIC_EVENT", testExecHistoryWorkerKey))
	return runCmd
}

func TestWorkerExecutionHistory_FormatJSON(t *testing.T) {
	entry, workerHistory := testExecutionHistoryWorkerHistory(t)
	runCmd := setupExecutionHistoryFormatTest(t, workerHistory)

	var out bytes.Buffer
	common.SetCliOut(&out)
	t.Cleanup(func() { common.SetCliOut(os.Stdout) })

	require.NoError(t, runCmd("worker", "execution-history", "--"+format.FlagName, "json"))
	assert.True(t, json.Valid(out.Bytes()), "expected valid JSON output, got: %s", out.String())
	assert.Contains(t, out.String(), entry.Result.Result)
}

func TestWorkerExecutionHistory_FormatTable(t *testing.T) {
	_, workerHistory := testExecutionHistoryWorkerHistory(t)
	runCmd := setupExecutionHistoryFormatTest(t, workerHistory)

	var out bytes.Buffer
	common.SetCliOut(&out)
	t.Cleanup(func() { common.SetCliOut(os.Stdout) })

	require.NoError(t, runCmd("worker", "execution-history", "--"+format.FlagName, "table"))
	outputStr := out.String()
	assert.False(t, json.Valid([]byte(strings.TrimSpace(outputStr))), "table output should not be JSON, got: %s", outputStr)
	assert.Contains(t, outputStr, "OK", "expected result value in table output, got: %s", outputStr)
	assert.Contains(t, outputStr, "execution succeeded", "expected logs value in table output, got: %s", outputStr)
}

func TestWorkerExecutionHistory_FormatDefault(t *testing.T) {
	_, workerHistory := testExecutionHistoryWorkerHistory(t)
	runCmd := setupExecutionHistoryFormatTest(t, workerHistory)

	var out bytes.Buffer
	common.SetCliOut(&out)
	t.Cleanup(func() { common.SetCliOut(os.Stdout) })

	require.NoError(t, runCmd("worker", "execution-history"))
	assert.True(t, json.Valid(out.Bytes()), "default output should be JSON, got: %s", out.String())
}

func TestWorkerExecutionHistory_FormatUnsupported(t *testing.T) {
	_, workerHistory := testExecutionHistoryWorkerHistory(t)
	runCmd := setupExecutionHistoryFormatTest(t, workerHistory)

	err := runCmd("worker", "execution-history", "--"+format.FlagName, "sarif")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

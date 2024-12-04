//go:build test
// +build test

package commands

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func TestExecute(t *testing.T) {
	payload := map[string]any{"my": "payload"}

	tests := []struct {
		name        string
		commandArgs []string
		assert      common.AssertOutputFunc
		action      string
		workerKey   string
		// The server behavior
		serverStub *common.ServerStub
		// If provided the cliIn will be filled with this content
		stdInput string
		// If provided a temp file will be generated with this content and the file path will be added at the end of the command
		fileInput     string
		patchManifest func(mf *model.Manifest)
	}{
		{
			name: "execute from manifest",
			serverStub: common.NewServerStub(t).
				WithExecuteEndpoint(common.ValidateJson(payload), payload),
			commandArgs: []string{common.MustJsonMarshal(t, payload)},
			assert:      common.AssertOutputJson(payload),
		},
		{
			name:      "execute with workerKey",
			workerKey: "my-worker",
			serverStub: common.NewServerStub(t).
				WithExecuteEndpoint(common.ValidateJson(payload), payload),
			commandArgs: []string{"my-worker", common.MustJsonMarshal(t, payload)},
			assert:      common.AssertOutputJson(payload),
		},
		{
			name:        "fails if not a GENERIC_EVENT",
			action:      "BEFORE_DOWNLOAD",
			serverStub:  common.NewServerStub(t),
			commandArgs: []string{`{}`},
			assert:      common.AssertOutputError("only the GENERIC_EVENT actions are executable. Got BEFORE_DOWNLOAD"),
		},
		{
			name:        "fails if not OK status",
			serverStub:  common.NewServerStub(t).WithToken("invalid-token").WithExecuteEndpoint(nil, nil),
			commandArgs: []string{`{}`},
			assert:      common.AssertOutputErrorRegexp(`command\sPOST.+returned\san\sunexpected\sstatus\scode\s403`),
		},
		{
			name:     "reads from stdin",
			stdInput: common.MustJsonMarshal(t, map[string]any{"my": "request"}),
			serverStub: common.NewServerStub(t).
				WithExecuteEndpoint(
					common.ValidateJson(map[string]any{"my": "request"}),
					map[string]any{"valid": "response"},
				),
			commandArgs: []string{"-"},
			assert:      common.AssertOutputJson(map[string]any{"valid": "response"}),
		},
		{
			name:      "reads from file",
			fileInput: common.MustJsonMarshal(t, map[string]any{"my": "file-content"}),
			serverStub: common.NewServerStub(t).
				WithExecuteEndpoint(
					common.ValidateJson(map[string]any{"my": "file-content"}),
					map[string]any{"valid": "response"},
				),
			assert: common.AssertOutputJson(map[string]any{"valid": "response"}),
		},
		{
			name:      "should propagate projectKey",
			workerKey: "my-worker",
			serverStub: common.NewServerStub(t).
				WithProjectKey("my-project").
				WithExecuteEndpoint(nil, payload),
			commandArgs: []string{"-"},
			stdInput:    `{}`,
			patchManifest: func(mf *model.Manifest) {
				mf.ProjectKey = "my-project"
				mf.Name = "my-worker"
			},
			assert: common.AssertOutputJson(payload),
		},
		{
			name:        "fails if invalid json from argument",
			commandArgs: []string{`{"my":`},
			assert:      common.AssertOutputError("invalid json payload: unexpected end of JSON input"),
		},
		{
			name:      "fails if invalid json from file argument",
			fileInput: `{"my":`,
			assert:    common.AssertOutputError("invalid json payload: unexpected end of JSON input"),
		},
		{
			name:        "fails if invalid json from standard input",
			commandArgs: []string{"-"},
			stdInput:    `{"my":`,
			assert:      common.AssertOutputError("unexpected EOF"),
		},
		{
			name:        "fails if missing file",
			commandArgs: []string{"@non-existing-file.json"},
			assert:      common.AssertOutputError("open non-existing-file.json: no such file or directory"),
		},
		{
			name:        "fails if timeout exceeds",
			commandArgs: []string{"--" + model.FlagTimeout, "500", `{}`},
			serverStub:  common.NewServerStub(t).WithDelay(5*time.Second).WithExecuteEndpoint(nil, nil),
			assert:      common.AssertOutputError("request timed out after 500ms"),
		},
		{
			name:        "fails if invalid timeout",
			commandArgs: []string{"--" + model.FlagTimeout, "abc", `{}`},
			assert:      common.AssertOutputError("invalid timeout provided"),
		},
		{
			name:        "fails if empty file path",
			commandArgs: []string{"@"},
			assert:      common.AssertOutputError("missing file path"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCmd := common.CreateCliRunner(t, GetInitCommand(), GetExecuteCommand())

			_, workerName := common.PrepareWorkerDirForTest(t)

			if tt.workerKey != "" {
				workerName = tt.workerKey
			}

			action := "GENERIC_EVENT"
			if tt.action != "" {
				action = tt.action
			}

			err := runCmd("worker", "init", action, workerName)
			require.NoError(t, err)

			if tt.patchManifest != nil {
				common.PatchManifest(t, tt.patchManifest)
			}

			if tt.serverStub == nil {
				tt.serverStub = common.NewServerStub(t)
			}

			common.NewMockWorkerServer(t,
				tt.serverStub.
					WithT(t).
					WithDefaultActionsMetadataEndpoint().
					WithGetOneEndpoint().
					WithWorkers(&model.WorkerDetails{
						Key: workerName,
					}),
			)

			if tt.stdInput != "" {
				common.SetCliIn(bytes.NewReader([]byte(tt.stdInput)))
				t.Cleanup(func() {
					common.SetCliIn(os.Stdin)
				})
			}

			if tt.fileInput != "" {
				tt.commandArgs = append(tt.commandArgs, "@"+common.CreateTempFileWithContent(t, tt.fileInput))
			}

			var output bytes.Buffer

			common.SetCliOut(&output)
			t.Cleanup(func() {
				common.SetCliOut(os.Stdout)
			})

			cmd := append([]string{"worker", "execute"}, tt.commandArgs...)

			err = runCmd(cmd...)

			tt.assert(t, output.Bytes(), err)
		})
	}
}

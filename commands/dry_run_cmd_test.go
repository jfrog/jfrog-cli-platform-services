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

func TestDryRun(t *testing.T) {
	tests := []struct {
		name          string
		commandArgs   []string
		initExtraArgs []string
		assert        common.AssertOutputFunc
		patchManifest func(mf *model.Manifest)
		// Use this workerKey instead of a random generated one
		workerKey string
		// The server behavior
		serverStub *common.ServerStub
		// If provided the cliIn will be filled with this content
		stdInput string
		// If provided a temp file will be generated with this content and the file path will be added at the end of the command
		fileInput string
	}{
		{
			name: "nominal case",
			serverStub: common.NewServerStub(t).
				WithTestEndpoint(
					validateTestPayloadData(map[string]any{"my": "payload"}),
					map[string]any{"my": "payload"},
				),
			commandArgs: []string{common.MustJsonMarshal(t, map[string]any{"my": "payload"})},
			assert:      common.AssertOutputJson(map[string]any{"my": "payload"}),
		},
		{
			name: "fails if not OK status",
			serverStub: common.NewServerStub(t).
				WithToken("invalid-token").
				WithTestEndpoint(nil, nil),
			commandArgs: []string{`{}`},
			assert:      common.AssertOutputErrorRegexp(`command.*returned\san\sunexpected\sstatus\scode\s403`),
		},
		{
			name:     "reads from stdin",
			stdInput: common.MustJsonMarshal(t, map[string]any{"my": "request"}),
			serverStub: common.NewServerStub(t).
				WithTestEndpoint(
					validateTestPayloadData(map[string]any{"my": "request"}),
					map[string]any{"valid": "response"},
				),
			commandArgs: []string{"-"},
			assert:      common.AssertOutputJson(map[string]any{"valid": "response"}),
		},
		{
			name:      "reads from file",
			fileInput: common.MustJsonMarshal(t, map[string]any{"my": "file-content"}),
			serverStub: common.NewServerStub(t).
				WithTestEndpoint(
					validateTestPayloadData(map[string]any{"my": "file-content"}),
					map[string]any{"valid": "response"},
				),
			assert: common.AssertOutputJson(map[string]any{"valid": "response"}),
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
			serverStub:  common.NewServerStub(t).WithDelay(2*time.Second).WithTestEndpoint(nil, nil),
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
		{
			name:          "should propagate projectKey",
			workerKey:     "my-worker",
			initExtraArgs: []string{"--" + model.FlagProjectKey, "my-project"},
			serverStub: common.NewServerStub(t).
				WithProjectKey("my-project").
				WithTestEndpoint(
					validateTestPayloadData(map[string]any{}),
					map[string]any{"valid": "response"},
				),
			commandArgs: []string{"-"},
			stdInput:    `{}`,
			patchManifest: func(mf *model.Manifest) {
				mf.ProjectKey = "my-project"
				mf.Name = "my-worker"
			},
			assert: common.AssertOutputJson(map[string]any{"valid": "response"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCmd := common.CreateCliRunner(t, GetInitCommand(), GetDryRunCommand())

			_, workerName := common.PrepareWorkerDirForTest(t)

			if tt.workerKey != "" {
				workerName = tt.workerKey
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

			initCmd := append([]string{"worker", "init"}, tt.initExtraArgs...)
			err := runCmd(append(initCmd, "BEFORE_DOWNLOAD", workerName)...)
			if err != nil {
				tt.assert(t, nil, err)
				return
			}

			if tt.patchManifest != nil {
				common.PatchManifest(t, tt.patchManifest)
			}

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

			cmd := append([]string{"worker", "dry-run"}, tt.commandArgs...)

			err = runCmd(cmd...)

			tt.assert(t, output.Bytes(), err)
		})
	}
}

func validateTestPayloadData(data any) common.BodyValidator {
	return common.ValidateJsonFunc(data, func(in any) any {
		var gotData any
		if m, isMap := data.(map[string]any); isMap {
			gotData = m
		}
		return gotData
	})
}

const workerKeyForDryRunTest = "test-worker"

func setupDryRunFormatTest(t *testing.T) (func(args ...string) error, *bytes.Buffer) {
	t.Helper()

	workerKey := workerKeyForDryRunTest
	serverStub := common.NewServerStub(t).
		WithWorkers(&model.WorkerDetails{Key: workerKey}).
		WithDefaultActionsMetadataEndpoint().
		WithGetOneEndpoint().
		WithTestEndpoint(nil, map[string]any{"status": "OK", "result": "done"})
	common.NewMockWorkerServer(t, serverStub)

	common.PrepareWorkerDirForTest(t)

	runCmd := common.CreateCliRunner(t, GetInitCommand(), GetDryRunCommand())
	require.NoError(t, runCmd("worker", "init", "BEFORE_DOWNLOAD", workerKey))

	var out bytes.Buffer
	common.SetCliOut(&out)
	t.Cleanup(func() { common.SetCliOut(os.Stdout) })

	return runCmd, &out
}

func TestWorkerDryRun_FormatJSON(t *testing.T) {
	runCmd, out := setupDryRunFormatTest(t)

	require.NoError(t, runCmd("worker", "dry-run", "--"+format.FlagName, "json", `{}`))
	assert.True(t, json.Valid(out.Bytes()), "expected valid JSON output, got: %s", out.String())
}

func TestWorkerDryRun_FormatTable(t *testing.T) {
	runCmd, out := setupDryRunFormatTest(t)

	require.NoError(t, runCmd("worker", "dry-run", "--"+format.FlagName, "table", `{}`))
	outputStr := out.String()
	assert.True(t, strings.Contains(outputStr, "status") || strings.Contains(outputStr, "result"),
		"expected table output to contain response fields, got: %s", outputStr)
}

func TestWorkerDryRun_FormatDefault(t *testing.T) {
	runCmd, out := setupDryRunFormatTest(t)

	require.NoError(t, runCmd("worker", "dry-run", `{}`))
	assert.True(t, json.Valid(out.Bytes()), "default output should be valid JSON, got: %s", out.String())
}

func TestWorkerDryRun_FormatUnsupported(t *testing.T) {
	runCmd, _ := setupDryRunFormatTest(t)

	err := runCmd("worker", "dry-run", "--"+format.FlagName, "sarif", `{}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only the following output formats are supported")
}

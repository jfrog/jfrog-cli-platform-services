//go:build itest

package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/jfrog/jfrog-cli-platform-services/test/infra"
)

type executeTestCase struct {
	name        string
	action      string
	workerKey   string
	only        bool
	skip        bool
	commandArgs []string
	assert      executeAssertFunc
	stdInput    string
	fileInput   string
}

type executeAssertFunc func(t *infra.Test, err error, tc *executeTestCase)

func TestExecute(t *testing.T) {
	infra.RunITests([]infra.TestDefinition{
		executeSpec(executeTestCase{
			name: "execute from manifest",
			commandArgs: []string{
				infra.MustJsonMarshal(t, map[string]any{"my": "payload"}),
			},
			assert: assertExecuteSucceed,
		}),
		executeSpec(executeTestCase{
			name:      "execute with key",
			workerKey: "my-worker",
			commandArgs: []string{
				"my-worker",
				infra.MustJsonMarshal(t, map[string]any{"my": "payload"}),
			},
			assert: assertExecuteSucceed,
		}),
		executeSpec(executeTestCase{
			name:        "reads from stdin",
			stdInput:    infra.MustJsonMarshal(t, map[string]any{"my": "request"}),
			commandArgs: []string{"-"},
			assert:      assertExecuteSucceed,
		}),
		executeSpec(executeTestCase{
			name:      "reads from file",
			fileInput: infra.MustJsonMarshal(t, map[string]any{"my": "file-content"}),
			assert:    assertExecuteSucceed,
		}),
		executeSpec(executeTestCase{
			name:        "fail if not a GENERIC_EVENT",
			action:      "BEFORE_DOWNLOAD",
			commandArgs: []string{`{}`},
			assert:      assertExecuteFail("only the GENERIC_EVENT actions are executable. Got BEFORE_DOWNLOAD"),
		}),
		executeSpec(executeTestCase{
			name:        "fails if invalid json from argument",
			commandArgs: []string{`{"my":`},
			assert:      assertExecuteFail("invalid json payload: unexpected end of JSON input"),
		}),
		executeSpec(executeTestCase{
			name:      "fails if invalid json from file argument",
			fileInput: `{"my":`,
			assert:    assertExecuteFail("invalid json payload: unexpected end of JSON input"),
		}),
		executeSpec(executeTestCase{
			name:        "fails if invalid json from standard input",
			commandArgs: []string{"-"},
			stdInput:    `{"my":`,
			assert:      assertExecuteFail("unexpected EOF"),
		}),
		executeSpec(executeTestCase{
			name:        "fails if invalid file path",
			commandArgs: []string{"@non-existing-file.json"},
			assert:      assertExecuteFail("open non-existing-file.json: no such file or directory"),
		}),
		executeSpec(executeTestCase{
			name:        "fails if invalid timeout",
			commandArgs: []string{"--" + model.FlagTimeout, "abc", `{}`},
			assert:      assertExecuteFail("invalid timeout provided"),
		}),
		executeSpec(executeTestCase{
			name:        "fails if empty file path",
			commandArgs: []string{"@"},
			assert:      assertExecuteFail("missing file path"),
		}),
	}, t)
}

func executeSpec(tc executeTestCase) infra.TestDefinition {
	return infra.TestDefinition{
		Name:          tc.name,
		Input:         tc.stdInput,
		Only:          tc.only,
		Skip:          tc.skip,
		CaptureOutput: true,
		Test: func(it *infra.Test) {
			workerDir, workerName := it.PrepareWorkerTestDir()

			if tc.workerKey != "" {
				workerName = tc.workerKey
			}

			action := "GENERIC_EVENT"
			if tc.action != "" {
				action = tc.action
			}

			err := it.RunCommand(infra.AppName, "init", action, workerName)
			require.NoError(it, err)

			if action == "GENERIC_EVENT" {
				// We make a generic event that returns its input
				err = os.WriteFile(
					filepath.Join(workerDir, "worker.ts"),
					[]byte(`export default async (context: PlatformContext, input: any) => input;`),
					os.ModePerm,
				)
				require.NoError(it, err)
			}

			// We should enable the worker
			infra.PatchManifest(it, func(mf *model.Manifest) {
				mf.Name = workerName
				mf.Enabled = true
			})

			// We should deploy the worker
			err = it.RunCommand(infra.AppName, "deploy")
			require.NoError(it, err)

			it.Cleanup(func() {
				it.DeleteWorker(workerName)
			})

			cmd := []string{infra.AppName, "execute"}
			cmd = append(cmd, tc.commandArgs...)

			if tc.fileInput != "" {
				cmd = append(cmd, "@"+infra.CreateTempFileWithContent(it, tc.fileInput))
			}

			tc.assert(it, it.RunCommand(cmd...), &tc)
		},
	}
}

func assertExecuteSucceed(it *infra.Test, err error, tc *executeTestCase) {
	require.NoError(it, err)

	gotResponse := map[string]any{}
	err = json.Unmarshal(it.CapturedOutput(), &gotResponse)
	require.NoError(it, err)

	// The worker returns its input
	wantResponse := map[string]any{}
	if tc.fileInput != "" {
		err = json.Unmarshal([]byte(tc.fileInput), &wantResponse)
		require.NoError(it, err)
	} else if tc.stdInput != "" {
		err = json.Unmarshal([]byte(tc.stdInput), &wantResponse)
		require.NoError(it, err)
	} else {
		require.True(it, len(tc.commandArgs) >= 1)
		err = json.Unmarshal([]byte(tc.commandArgs[len(tc.commandArgs)-1]), &wantResponse)
		require.NoError(it, err)
	}

	assert.Equal(it, map[string]any{
		"data":            wantResponse,
		"executionStatus": "STATUS_SUCCESS",
	}, gotResponse)
}

func assertExecuteFail(errorMessage string, errorMessageArgs ...any) executeAssertFunc {
	return func(it *infra.Test, err error, tc *executeTestCase) {
		require.Error(it, err)
		assert.EqualError(it, err, fmt.Sprintf(errorMessage, errorMessageArgs...))
	}
}

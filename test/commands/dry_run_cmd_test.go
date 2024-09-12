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

type dryRunTestCase struct {
	name          string
	only          bool
	skip          bool
	commandArgs   []string
	assert        dryRunAssertFunc
	stdInput      string
	fileInput     string
	code          string
	initWorkers   []*model.WorkerDetails
	patchManifest func(mf *model.Manifest)
}

type dryRunAssertFunc func(t *infra.Test, err error, tc *dryRunTestCase)

func TestDryRun(t *testing.T) {
	infra.RunITests([]infra.TestDefinition{
		dryRunSpec(dryRunTestCase{
			name: "nominal case",
			commandArgs: []string{
				infra.MustJsonMarshal(t, map[string]any{"my": "payload"}),
			},
			assert: assertDryRunSucceed,
		}),
		dryRunSpec(dryRunTestCase{
			name:        "reads from stdin",
			stdInput:    infra.MustJsonMarshal(t, map[string]any{"my": "request"}),
			commandArgs: []string{"-"},
			assert:      assertDryRunSucceed,
		}),
		dryRunSpec(dryRunTestCase{
			name:      "reads from file",
			fileInput: infra.MustJsonMarshal(t, map[string]any{"my": "file-content"}),
			assert:    assertDryRunSucceed,
		}),
		dryRunSpec(dryRunTestCase{
			name:        "fails if invalid json from argument",
			commandArgs: []string{`{"my":`},
			assert:      assertDryRunFail("invalid json payload: unexpected end of JSON input"),
		}),
		dryRunSpec(dryRunTestCase{
			name:      "fails if invalid json from file argument",
			fileInput: `{"my":`,
			assert:    assertDryRunFail("invalid json payload: unexpected end of JSON input"),
		}),
		dryRunSpec(dryRunTestCase{
			name:        "fails if invalid json from standard input",
			commandArgs: []string{"-"},
			stdInput:    `{"my":`,
			assert:      assertDryRunFail("unexpected EOF"),
		}),
		dryRunSpec(dryRunTestCase{
			name:        "fails if invalid file path",
			commandArgs: []string{"@non-existing-file.json"},
			assert:      assertDryRunFail("open non-existing-file.json: no such file or directory"),
		}),
		dryRunSpec(dryRunTestCase{
			name:        "fails if invalid timeout",
			commandArgs: []string{"--" + model.FlagTimeout, "abc", `{}`},
			assert:      assertDryRunFail("invalid timeout provided"),
		}),
		dryRunSpec(dryRunTestCase{
			name:        "fails if empty file path",
			commandArgs: []string{"@"},
			assert:      assertDryRunFail("missing file path"),
		}),
		dryRunSpec(dryRunTestCase{
			name:        "with project and secrets update",
			commandArgs: []string{"-"},
			stdInput:    "{}",
			code:        "export default async (context) => ({ 'check': context.secrets.get('sec-1') === 'val-1-updated' })",
			initWorkers: []*model.WorkerDetails{
				{
					Key:         "wk-1",
					Description: "My worker",
					Enabled:     true,
					SourceCode:  `export default async function() { return { "status": "OK" } }`,
					Action:      model.ActionGenericEvent,
					Secrets: []*model.Secret{
						{
							Key: "sec-1", Value: "val-1",
						},
					},
					ProjectKey: "my-project",
				},
			},
			patchManifest: func(mf *model.Manifest) {
				mf.ProjectKey = "my-project"
				mf.Name = "wk-1"
				mf.Secrets = model.Secrets{"sec-1": infra.MustEncryptSecret(t, "val-1-updated")}
			},
			assert: assertDryRunWithSecretsUpdate,
		}),
	}, t)
}

func dryRunSpec(tc dryRunTestCase) infra.TestDefinition {
	return infra.TestDefinition{
		Name:          tc.name,
		Input:         tc.stdInput,
		Only:          tc.only,
		Skip:          tc.skip,
		CaptureOutput: true,
		Test: func(it *infra.Test) {
			for _, initialWorker := range tc.initWorkers {
				it.CreateWorker(initialWorker)
				it.Cleanup(func() {
					it.DeleteWorker(initialWorker.KeyWithProject())
				})
			}

			workerDir, workerName := it.PrepareWorkerTestDir()

			err := it.RunCommand(infra.AppName, "init", "GENERIC_EVENT", workerName)
			require.NoError(it, err)

			// We make a generic event that returns its input as default code
			code := `export default async (context: PlatformContext, input: any) => input;`
			if tc.code != "" {
				code = tc.code
			}

			err = os.WriteFile(filepath.Join(workerDir, "worker.ts"), []byte(code), os.ModePerm)
			require.NoError(it, err)

			infra.AddSecretPasswordToEnv(it)

			if tc.patchManifest != nil {
				infra.PatchManifest(it, tc.patchManifest)
			}

			cmd := []string{infra.AppName, "dry-run"}
			cmd = append(cmd, tc.commandArgs...)

			if tc.fileInput != "" {
				cmd = append(cmd, "@"+infra.CreateTempFileWithContent(it, tc.fileInput))
			}

			tc.assert(it, it.RunCommand(cmd...), &tc)
		},
	}
}

func assertDryRunSucceed(it *infra.Test, err error, tc *dryRunTestCase) {
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
		require.Len(it, tc.commandArgs, 1)
		err = json.Unmarshal([]byte(tc.commandArgs[0]), &wantResponse)
		require.NoError(it, err)
	}

	gotGenericEvent, ok := gotResponse["genericEvent"]
	require.Truef(it, ok, "Generic not found in response")

	assert.Equal(it,
		map[string]any{
			"data":            wantResponse,
			"executionStatus": "STATUS_SUCCESS",
		}, gotGenericEvent)
}

func assertDryRunFail(errorMessage string, errorMessageArgs ...any) dryRunAssertFunc {
	return func(it *infra.Test, err error, tc *dryRunTestCase) {
		require.Error(it, err)
		assert.EqualError(it, err, fmt.Sprintf(errorMessage, errorMessageArgs...))
	}
}

func assertDryRunWithSecretsUpdate(it *infra.Test, err error, tc *dryRunTestCase) {
	require.NoError(it, err)

	check := struct {
		Event struct {
			Data struct {
				V bool `json:"check"`
			} `json:"data"`
		} `json:"genericEvent"`
	}{}

	err = json.Unmarshal(it.CapturedOutput(), &check)
	require.NoError(it, err)

	assert.Truef(it, check.Event.Data.V, "Dry run did not apply secret update")
}

//go:build test
// +build test

package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func TestListCommand(t *testing.T) {
	tests := []struct {
		name        string
		commandArgs []string
		token       string
		serverStub  *common.ServerStub
		assert      common.AssertOutputFunc
	}{
		{
			name: "list",
			serverStub: common.NewServerStub(t).
				WithGetAllEndpoint().
				WithWorkers([]*model.WorkerDetails{
					{
						Key:         "wk-0",
						Action:      "AFTER_CREATE",
						Description: "run wk-0",
						Enabled:     true,
						SourceCode:  "export default async () => ({ 'S': 'OK'})",
					},
				}...),
			assert: common.AssertOutputText("wk-0,AFTER_CREATE,run wk-0,true", "invalid csv received"),
		},
		{
			name:        "list worker of type",
			commandArgs: []string{"AFTER_CREATE"},
			serverStub: common.NewServerStub(t).
				WithGetAllEndpoint().
				WithWorkers([]*model.WorkerDetails{
					{
						Key:         "wk-0",
						Action:      "AFTER_CREATE",
						Description: "run wk-0",
						Enabled:     true,
						SourceCode:  "export default async () => ({ 'S': 'OK'})",
					},
					{
						Key:         "wk-1",
						Action:      "BEFORE_DOWNLOAD",
						Description: "run wk-1",
						Enabled:     true,
						SourceCode:  "export default async () => ({ 'S': 'OK'})",
					},
				}...),
			assert: common.AssertOutputText("wk-0,AFTER_CREATE,run wk-0,true", "invalid csv received"),
		},
		{
			name:        "list for JSON",
			commandArgs: []string{"--" + model.FlagJSONOutput},
			serverStub: common.NewServerStub(t).
				WithGetAllEndpoint().
				WithWorkers([]*model.WorkerDetails{
					{
						Key:         "wk-1",
						Action:      "GENERIC_EVENT",
						Description: "run wk-1",
						Enabled:     false,
						SourceCode:  "export default async () => ({ 'S': 'OK'})",
					},
				}...),
			assert: assertWorkerListOutput([]*model.WorkerDetails{
				{
					Key:         "wk-1",
					Action:      "GENERIC_EVENT",
					Description: "run wk-1",
					Enabled:     false,
					SourceCode:  "export default async () => ({ 'S': 'OK'})",
				},
			}),
		},
		{
			name:        "projectKey is passed to the request",
			commandArgs: []string{"--" + model.FlagProjectKey, "my-project", "--" + model.FlagJSONOutput},
			serverStub:  common.NewServerStub(t).WithProjectKey("my-project").WithGetAllEndpoint(),
			assert:      common.AssertOutputJson(map[string]any{"workers": []any{}}),
		},
		{
			name:        "fails if timeout exceeds",
			commandArgs: []string{"--" + model.FlagTimeout, "500", `{}`},
			serverStub:  common.NewServerStub(t).WithDelay(5 * time.Second).WithGetAllEndpoint(),
			assert:      common.AssertOutputError("request timed out after 500ms"),
		},
		{
			name:        "fails if invalid timeout",
			commandArgs: []string{"--" + model.FlagTimeout, "abc"},
			assert:      common.AssertOutputError("invalid timeout provided"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.serverStub == nil {
				tt.serverStub = common.NewServerStub(t)
			}

			common.NewMockWorkerServer(t, tt.serverStub.WithT(t).WithDefaultActionsMetadataEndpoint())

			runCmd := common.CreateCliRunner(t, GetListCommand())

			var output bytes.Buffer
			common.SetCliOut(&output)
			t.Cleanup(func() {
				common.SetCliOut(os.Stdout)
			})

			cmd := append([]string{"worker", "list"}, tt.commandArgs...)

			err := runCmd(cmd...)

			tt.assert(t, output.Bytes(), err)
		})
	}
}

func assertWorkerListOutput(want []*model.WorkerDetails) common.AssertOutputFunc {
	return func(t *testing.T, output []byte, err error) {
		require.NoError(t, err)

		var got getAllResponse

		err = json.Unmarshal(output, &got)
		require.NoError(t, err)

		sortWorkers(got.Workers)
		sortWorkers(want)

		assert.Equal(t, want, got.Workers)
	}
}

func sortWorkers(workers []*model.WorkerDetails) {
	sort.Slice(workers, func(i, j int) bool {
		return workers[i].Key < workers[j].Key
	})
}

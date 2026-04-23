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

func TestListEventCommand(t *testing.T) {
	tests := []struct {
		name        string
		commandArgs []string
		serverStub  *common.ServerStub
		assert      common.AssertOutputFunc
	}{
		{
			name:       "list",
			serverStub: common.NewServerStub(t).WithDefaultActionsMetadataEndpoint(),
			assert:     common.AssertOutputText(strings.Join(common.LoadSampleActionEvents(t), ", "), "invalid data "),
		},
		{
			name:        "fails if timeout exceeds",
			serverStub:  common.NewServerStub(t).WithDelay(2 * time.Second).WithDefaultActionsMetadataEndpoint(),
			commandArgs: []string{"--" + model.FlagTimeout, "500"},
			assert:      common.AssertOutputError("request timed out after 500ms"),
		},
		{
			name:        "should propagate projectKey",
			serverStub:  common.NewServerStub(t).WithProjectKey("proj-1").WithDefaultActionsMetadataEndpoint(),
			commandArgs: []string{"wk-1", "--" + model.FlagProjectKey, "proj-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCmd := common.CreateCliRunner(t, GetListEventsCommand())

			common.NewMockWorkerServer(t, tt.serverStub.WithT(t))

			var output bytes.Buffer
			common.SetCliOut(&output)
			t.Cleanup(func() {
				common.SetCliOut(os.Stdout)
			})

			cmd := append([]string{"worker", "list-event"}, tt.commandArgs...)

			err := runCmd(cmd...)

			if tt.assert == nil {
				require.NoError(t, err)
			} else {
				tt.assert(t, output.Bytes(), err)
			}
		})
	}
}

func TestWorkerListEvent_FormatJSON(t *testing.T) {
	serverStub := common.NewServerStub(t).WithDefaultActionsMetadataEndpoint()
	common.NewMockWorkerServer(t, serverStub)

	var out bytes.Buffer
	common.SetCliOut(&out)
	t.Cleanup(func() { common.SetCliOut(os.Stdout) })

	runCmd := common.CreateCliRunner(t, GetListEventsCommand())
	require.NoError(t, runCmd("worker", "list-event", "--"+format.FlagName, "json"))
	assert.True(t, json.Valid(out.Bytes()), "expected valid JSON output, got: %s", out.String())
}

func TestWorkerListEvent_FormatTable(t *testing.T) {
	serverStub := common.NewServerStub(t).WithDefaultActionsMetadataEndpoint()
	common.NewMockWorkerServer(t, serverStub)

	var out bytes.Buffer
	common.SetCliOut(&out)
	t.Cleanup(func() { common.SetCliOut(os.Stdout) })

	runCmd := common.CreateCliRunner(t, GetListEventsCommand())
	require.NoError(t, runCmd("worker", "list-event", "--"+format.FlagName, "table"))

	outputStr := out.String()
	sampleEvents := common.LoadSampleActionEvents(t)
	for _, event := range sampleEvents {
		assert.True(t, strings.Contains(outputStr, event), "expected event %q in table output, got: %s", event, outputStr)
	}
}

func TestWorkerListEvent_FormatDefault(t *testing.T) {
	serverStub := common.NewServerStub(t).WithDefaultActionsMetadataEndpoint()
	common.NewMockWorkerServer(t, serverStub)

	var outDefault bytes.Buffer
	common.SetCliOut(&outDefault)
	t.Cleanup(func() { common.SetCliOut(os.Stdout) })

	runCmd := common.CreateCliRunner(t, GetListEventsCommand())
	require.NoError(t, runCmd("worker", "list-event"))

	// Default output (no --format flag) should be plain text (not JSON)
	assert.False(t, json.Valid(outDefault.Bytes()), "default output should not be JSON, got: %s", outDefault.String())

	sampleEvents := common.LoadSampleActionEvents(t)
	assert.Equal(t, strings.Join(sampleEvents, ", "), strings.TrimSpace(outDefault.String()))
}

func TestWorkerListEvent_FormatUnsupported(t *testing.T) {
	serverStub := common.NewServerStub(t).WithDefaultActionsMetadataEndpoint()
	common.NewMockWorkerServer(t, serverStub)

	runCmd := common.CreateCliRunner(t, GetListEventsCommand())
	err := runCmd("worker", "list-event", "--"+format.FlagName, "sarif")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

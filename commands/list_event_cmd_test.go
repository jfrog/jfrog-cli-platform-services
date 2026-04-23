//go:build test
// +build test

package commands

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"
	"github.com/jfrog/jfrog-cli-platform-services/model"
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

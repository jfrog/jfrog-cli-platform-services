//go:build test
// +build test

package commands

import (
	"errors"
	"testing"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditScheduleCommand(t *testing.T) {
	tests := []struct {
		name         string
		cron         string
		timezone     string
		workerAction string
		wantErr      error
	}{
		{
			name:     "edit schedule",
			cron:     "0 1 * * *",
			timezone: "America/New_York",
		},
		{
			name:     "edit schedule with empty cron",
			timezone: "UTC",
			wantErr:  errors.New("Mandatory flag 'cron' is missing"),
		},
		{
			name:     "edit schedule with invalid cron",
			cron:     "0 1 * * * * * *",
			timezone: "America/New_York",
			wantErr:  errors.New("invalid schedule provided: invalid cron expression"),
		},
		{
			name:     "edit schedule with invalid timezone",
			cron:     "0 1 * * *",
			timezone: "Asia/Chicago",
			wantErr:  errors.New("invalid schedule provided: invalid timezone 'Asia/Chicago'"),
		},
		{
			name:         "edit schedule for non-scheduled worker",
			cron:         "0 1 * * *",
			workerAction: "GENERIC_EVENT",
			wantErr:      errors.New("the worker is not a SCHEDULED_EVENT worker"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			common.NewMockWorkerServer(t, common.NewServerStub(t).WithDefaultActionsMetadataEndpoint())

			runCmd := common.CreateCliRunner(t, GetInitCommand(), GetEditScheduleCommand())

			_, workerName := common.PrepareWorkerDirForTest(t)

			workerAction := tt.workerAction
			if workerAction == "" {
				workerAction = "SCHEDULED_EVENT"
			}

			err := runCmd("worker", "init", workerAction, workerName)
			require.NoError(t, err)

			var commandArgs []string
			if tt.cron != "" {
				commandArgs = append(commandArgs, "--"+flagScheduleCron, tt.cron)
			}
			if tt.timezone != "" {
				commandArgs = append(commandArgs, "--"+flagScheduleTimezone, tt.timezone)
			}

			cmd := append([]string{"worker", "edit-schedule"}, commandArgs...)

			err = runCmd(cmd...)

			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr.Error())
			}
		})
	}
}

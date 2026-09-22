//go:build test
// +build test

package commands

import (
	"fmt"
	"os"
	"testing"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"
	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPropertyCmd(t *testing.T) {
	tests := []struct {
		name          string
		commandArgs   []string
		propertyName  string
		propertyValue string
		wantErr       string
		want          map[string]string
		patchManifest func(mf *model.Manifest)
	}{
		{
			name:          "add",
			propertyName:  "prop-1",
			propertyValue: "value-1",
			patchManifest: func(mf *model.Manifest) {
				mf.Properties = map[string]string{"prop-2": "value-2"}
			},
			want: map[string]string{"prop-1": "value-1", "prop-2": "value-2"},
		},
		{
			name:        "add from argument",
			commandArgs: []string{"prop-1", "value-1"},
			patchManifest: func(mf *model.Manifest) {
				mf.Properties = map[string]string{"prop-2": "value-2"}
			},
			want: map[string]string{"prop-1": "value-1", "prop-2": "value-2"},
		},
		{
			name:          "argument overrides env",
			commandArgs:   []string{"prop-1", "from-arg"},
			propertyValue: "from-env",
			want:          map[string]string{"prop-1": "from-arg"},
		},
		{
			name:        "reject extra arguments",
			commandArgs: []string{"prop-1", "value-1", "extra"},
			wantErr:     "Wrong number of arguments (3).",
		},
		{
			name:          "add with nil properties",
			propertyName:  "prop-1",
			propertyValue: "value-1",
			patchManifest: func(mf *model.Manifest) {
				mf.Properties = nil
			},
			want: map[string]string{"prop-1": "value-1"},
		},
		{
			name:          "edit property",
			propertyName:  "prop-1",
			propertyValue: "new-value",
			commandArgs:   []string{fmt.Sprintf("--%s", model.FlagEdit)},
			patchManifest: func(mf *model.Manifest) {
				mf.Properties = map[string]string{"prop-1": "old-value"}
			},
			want: map[string]string{"prop-1": "new-value"},
		},
		{
			name:          "reject duplicate without edit",
			propertyName:  "prop-1",
			propertyValue: "new-value",
			patchManifest: func(mf *model.Manifest) {
				mf.Properties = map[string]string{"prop-1": "old-value"}
			},
			wantErr: "prop-1 already exists, use --edit to overwrite",
		},
		{
			name:    "reject missing name",
			wantErr: "Wrong number of arguments (0).",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			common.NewMockWorkerServer(t, common.NewServerStub(t).WithDefaultActionsMetadataEndpoint())

			workerDir, workerName := common.PrepareWorkerDirForTest(t)
			runCmd := common.CreateCliRunner(t, GetInitCommand(), GetAddPropertyCommand())
			require.NoError(t, runCmd("worker", "init", "GENERIC_EVENT", workerName))

			if tt.patchManifest != nil {
				common.PatchManifest(t, tt.patchManifest)
			}
			if tt.propertyValue != "" {
				require.NoError(t, os.Setenv(model.EnvKeyAddPropertyValue, tt.propertyValue))
				t.Cleanup(func() { _ = os.Unsetenv(model.EnvKeyAddPropertyValue) })
			}

			cmd := append([]string{"worker", "add-property"}, tt.commandArgs...)
			if tt.propertyName != "" {
				cmd = append(cmd, tt.propertyName)
			}

			err := runCmd(cmd...)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			manifest, err := common.ReadManifest(workerDir)
			require.NoError(t, err)
			assert.Equal(t, tt.want, manifest.Properties)
		})
	}
}

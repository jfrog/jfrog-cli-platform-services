//go:build test
// +build test

package common

import (
	"testing"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func Test_cleanImports(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "case 1",
			source: `import { a } from 'b'; export default async (context: a) => ({ status: 200 })`,
			want:   "export default async (context: a) => ({ status: 200 })",
		},
		{
			name: "case 2",
			source: `
				import { a } from 'b'; 
				import { c, d } from 'e';

				export default async (context: a) => ({ status: 200 })`,
			want: "export default async (context: a) => ({ status: 200 })",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanImports(tt.source)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_extractProjectAndKeyFromCommandContext(t *testing.T) {
	tests := []struct {
		name         string
		c            stringFlagAware
		args         []string
		minArguments int
		onlyGeneric  bool
		eventType    string
		assert       func(t *testing.T, manifestWorkerKey, manifestProjectKey, extractedWorkerKey, extractedProjectKey string, err error)
	}{
		{
			name:         "project and worker key from args",
			c:            mockStringFlagAware{model.FlagProjectKey: "proj1"},
			args:         []string{"worker1"},
			minArguments: 0,
			onlyGeneric:  false,
			assert: func(t *testing.T, _, _, extractedWorkerKey, extractedProjectKey string, err error) {
				require.NoError(t, err)
				assert.Equal(t, "worker1", extractedWorkerKey)
				assert.Equal(t, "proj1", extractedProjectKey)
			},
		},
		{
			name:         "project and worker key from manifest",
			c:            mockStringFlagAware{},
			args:         []string{},
			minArguments: 0,
			onlyGeneric:  false,
			assert: func(t *testing.T, manifestWorkerKey, manifestProjectKey, extractedWorkerKey, extractedProjectKey string, err error) {
				require.NoError(t, err)
				assert.Equal(t, manifestWorkerKey, extractedWorkerKey)
				assert.Equal(t, manifestProjectKey, extractedProjectKey)
			},
		},
		{
			name:         "only generic event allowed",
			c:            mockStringFlagAware{model.FlagProjectKey: ""},
			args:         []string{},
			minArguments: 0,
			onlyGeneric:  true,
			eventType:    "BEFORE_DOWNLOAD",
			assert: func(t *testing.T, _, _, _, _ string, err error) {
				assert.EqualError(t, err, "only the GENERIC_EVENT actions are executable. Got BEFORE_DOWNLOAD")
			},
		},
		{
			name:         "min arguments count not satisfied",
			c:            mockStringFlagAware{model.FlagProjectKey: ""},
			args:         []string{"@jsonPayload.json"},
			minArguments: 1,
			onlyGeneric:  false,
			assert: func(t *testing.T, manifestWorkerKey, manifestProjectKey, extractedWorkerKey, extractedProjectKey string, err error) {
				require.NoError(t, err)
				assert.Equal(t, manifestWorkerKey, extractedWorkerKey)
				assert.Equal(t, manifestProjectKey, extractedProjectKey)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, manifestWorkerKey := PrepareWorkerDirForTest(t)
			manifestProjectKey := uuid.NewString()

			mf := model.Manifest{
				Name:           manifestWorkerKey,
				ProjectKey:     manifestProjectKey,
				Action:         tt.eventType,
				SourceCodePath: "./worker.ts",
			}

			if mf.Action == "" {
				mf.Action = "GENERIC_EVENT"
			}

			require.NoError(t, SaveManifest(&mf, dir))

			workerKey, projectKey, err := ExtractProjectAndKeyFromCommandContext(tt.c, tt.args, tt.minArguments, tt.onlyGeneric)

			tt.assert(t, manifestWorkerKey, manifestProjectKey, workerKey, projectKey, err)
		})
	}
}

type mockStringFlagAware map[string]string

func (m mockStringFlagAware) GetStringFlagValue(flag string) string {
	return m[flag]
}

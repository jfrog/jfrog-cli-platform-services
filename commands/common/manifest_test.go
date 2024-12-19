//go:build test
// +build test

package common

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/jfrog/jfrog-cli-platform-services/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var manifestSample = &model.Manifest{
	Name:           "my-worker",
	Description:    "my worker description",
	SourceCodePath: "./my-worker.ts",
	Action:         "BEFORE_DOWNLOAD",
	Enabled:        true,
	Debug:          true,
	ProjectKey:     "a-project",
	Secrets: model.Secrets{
		"hidden1": "hidden1.value",
		"hidden2": "hidden2.value",
	},
	FilterCriteria: model.FilterCriteria{
		ArtifactFilterCriteria: model.ArtifactFilterCriteria{
			RepoKeys: []string{
				"my-repo-local",
			},
		},
	},
}

func TestReadManifest(t *testing.T) {
	tests := []struct {
		name     string
		dirAsArg bool
		sample   *model.Manifest
		assert   func(t *testing.T, mf *model.Manifest, readErr error)
	}{
		{
			name:   "in current dir",
			sample: manifestSample,
			assert: func(t *testing.T, mf *model.Manifest, readErr error) {
				require.NoError(t, readErr)
				assert.Equal(t, manifestSample, mf)
			},
		},
		{
			name:     "with dir as argument",
			sample:   manifestSample,
			dirAsArg: true,
			assert: func(t *testing.T, mf *model.Manifest, readErr error) {
				require.NoError(t, readErr)
				assert.Equal(t, manifestSample, mf)
			},
		},
		{
			name: "with missing manifest",
			assert: func(t *testing.T, mf *model.Manifest, readErr error) {
				require.Error(t, readErr)
				require.True(t, os.IsNotExist(readErr))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifestFolder, err := os.MkdirTemp("", "wks-cli-*.manifest")
			require.NoError(t, err)

			t.Cleanup(func() {
				// We do not care about this error
				_ = os.RemoveAll(manifestFolder)
			})

			if tt.sample != nil {
				manifestBytes, err := json.Marshal(tt.sample)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(manifestFolder, "manifest.json"), manifestBytes, os.ModePerm)
				require.NoError(t, err)
			}

			var dirParams []string
			if tt.dirAsArg {
				dirParams = append(dirParams, manifestFolder)
			} else {
				err = os.Chdir(manifestFolder)
				require.NoError(t, err)
			}

			mf, err := ReadManifest(dirParams...)

			tt.assert(t, mf, err)
		})
	}
}

func TestManifest_ReadSourceCode(t *testing.T) {
	tests := []struct {
		name       string
		sourceCode string
		manifest   *model.Manifest
		want       string
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "nominal case",
			manifest:   manifestSample,
			sourceCode: "export async () => ({ status: 'SUCCESS' })",
			want:       "export async () => ({ status: 'SUCCESS' })",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				assert.NoError(t, err)
				return err == nil
			},
		},
		{
			name:     "missing source file",
			manifest: manifestSample,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				assert.Error(t, err)
				assert.True(t, os.IsNotExist(err))
				return false
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifestFolder, err := os.MkdirTemp("", "wks-cli-*.source")
			require.NoError(t, err)

			t.Cleanup(func() {
				// We do not care about this error
				_ = os.RemoveAll(manifestFolder)
			})

			if tt.sourceCode != "" {
				err = os.WriteFile(filepath.Join(manifestFolder, tt.manifest.SourceCodePath), []byte(tt.sourceCode), os.ModePerm)
				require.NoError(t, err)
			}

			err = os.Chdir(manifestFolder)
			require.NoError(t, err)

			got, err := ReadSourceCode(manifestSample)
			if !tt.wantErr(t, err, "ReadSourceCode()") {
				return
			}

			assert.Equalf(t, tt.want, got, "ReadSourceCode()")
		})
	}
}

func TestManifest_Validate(t *testing.T) {
	sampleActions := LoadSampleActions(t)

	tests := []struct {
		name        string
		manifest    *model.Manifest
		assert      func(t *testing.T, err error)
		actionsMeta ActionsMetadata
	}{
		{
			name:        "valid",
			manifest:    manifestSample,
			actionsMeta: sampleActions,
			assert: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:        "missing name",
			actionsMeta: sampleActions,
			manifest: patchedManifestSample(func(mf *model.Manifest) {
				mf.Name = ""
			}),
			assert: func(t *testing.T, err error) {
				assert.EqualError(t, err, invalidManifestErr("missing name").Error())
			},
		},
		{
			name:        "missing source code path",
			actionsMeta: sampleActions,
			manifest: patchedManifestSample(func(mf *model.Manifest) {
				mf.SourceCodePath = ""
			}),
			assert: func(t *testing.T, err error) {
				assert.EqualError(t, err, invalidManifestErr("missing source code path").Error())
			},
		},
		{
			name:        "missing action",
			actionsMeta: sampleActions,
			manifest: patchedManifestSample(func(mf *model.Manifest) {
				mf.Action = ""
			}),
			assert: func(t *testing.T, err error) {
				assert.EqualError(t, err, invalidManifestErr("missing action").Error())
			},
		},
		{
			name:        "invalid action",
			actionsMeta: sampleActions,
			manifest: patchedManifestSample(func(mf *model.Manifest) {
				mf.Action = "HACK_ME"
			}),
			assert: func(t *testing.T, err error) {
				assert.Regexp(t, regexp.MustCompile("action 'HACK_ME' not found"), err)
			},
		},
		{
			name: "no action validation if no actions metadata",
			manifest: patchedManifestSample(func(mf *model.Manifest) {
				mf.Action = "HACK_ME"
			}),
			assert: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, ValidateManifest(tt.manifest, tt.actionsMeta))
		})
	}
}

func TestManifest_DecryptSecrets(t *testing.T) {
	tests := []struct {
		name            string
		encryptSecrets  model.Secrets
		verbatimSecrets model.Secrets
		assert          func(t *testing.T, mf *model.Manifest, err error)
	}{
		{
			name: "ok",
			encryptSecrets: model.Secrets{
				"s1": "v1",
				"s2": "v2",
			},
			assert: func(t *testing.T, mf *model.Manifest, err error) {
				require.NoError(t, err)
				assert.Equal(t, model.Secrets{
					"s1": "v1",
					"s2": "v2",
				}, mf.Secrets)
			},
		},
		{
			name: "with cleartext secrets",
			verbatimSecrets: model.Secrets{
				"s1": "v1",
			},
			assert: func(t *testing.T, mf *model.Manifest, err error) {
				assert.EqualError(t, err, "cannot decrypt secret 's1', please check the manifest")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.Setenv(model.EnvKeySecretsPassword, "P@ssw0rd!")
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.Unsetenv(model.EnvKeySecretsPassword)
			})

			mf := patchedManifestSample(func(mf *model.Manifest) {
				mf.Secrets = model.Secrets{}

				var err error
				for key, val := range tt.encryptSecrets {
					mf.Secrets[key], err = EncryptSecret("P@ssw0rd!", val)
					require.NoError(t, err)
				}

				for key, val := range tt.verbatimSecrets {
					mf.Secrets[key] = val
				}
			})

			tt.assert(t, mf, DecryptManifestSecrets(mf))
		})
	}
}

func TestManifest_ValidateScheduleCriteria(t *testing.T) {
	tests := []struct {
		name     string
		criteria *model.ScheduleFilterCriteria
		wantErr  error
	}{
		{
			name: "valid",
			criteria: &model.ScheduleFilterCriteria{
				Cron:     "0 1 1 * *",
				Timezone: "UTC",
			},
		},
		{
			name: "missing cron",
			criteria: &model.ScheduleFilterCriteria{
				Timezone: "UTC",
			},
			wantErr: errors.New("missing cron expression"),
		},
		{
			name: "invalid cron",
			criteria: &model.ScheduleFilterCriteria{
				Cron:     "0 0 0 * * * *",
				Timezone: "UTC",
			},
			wantErr: errors.New("invalid cron expression"),
		},
		{
			name: "invalid timezone",
			criteria: &model.ScheduleFilterCriteria{
				Cron:     "0 1 1 * *",
				Timezone: "America/Toulouse",
			},
			wantErr: errors.New("invalid timezone 'America/Toulouse'"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScheduleCriteria(tt.criteria)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr.Error())
			}
		})
	}
}

func patchedManifestSample(patch func(mf *model.Manifest)) *model.Manifest {
	patched := *manifestSample
	patch(&patched)
	return &patched
}
